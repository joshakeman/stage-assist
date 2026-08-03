# Stage Assist

Compares an original theatrical script against a rehearsal transcript and
produces "line notes" — feedback showing an actor where their delivered
lines diverged from the script. Also a portfolio project for practicing
idiomatic Go + React/TypeScript; code quality matters as much as features.

## Architecture

Go backend (`backend/`, stdlib `net/http`, no framework) + React/TypeScript
frontend (`frontend/`, Vite). The frontend calls only the Go backend; it
never calls Anthropic directly, and no Anthropic API key is ever present in
the frontend bundle. **Any future Claude/Anthropic integration must be
proxied through the Go backend.**

### Parsing vs. comparison boundary

Raw text becomes line notes through three stages, each ignorant of the
stage before it:

`ParsePlainTextScript` (`internal/domain/script.go`) → `Script` →
`ExtractCues` (`internal/domain/cues.go`) → `Align`/`WordDiff`.

`ParsePlainTextScript` knows about source *format* (today: the `CHARACTER:`
colon convention) and nothing else — it has no idea which character will
later be extracted. `ExtractCues` is the only place that knows about
"character" — it walks a `Script` and pulls one speaker's ordered dialogue
out of it, with no idea what format produced that `Script`. Everything
downstream of that (`Align`, `WordDiff`) operates on plain ordered
`Cue`/`Token` sequences with no notion of character, speaker, or format.
Keep all three ignorant of each other: adding a format (Fountain, PDF,
DOCX) is a new `Parse<Format>Script` function only; supporting a different
extraction (e.g. a whole scene) is a change to `cues.go` only; neither
should ever require touching `align.go`/`diff.go`.

### Script model

`Script`/`ScriptElement` (`internal/domain/script.go`) is the canonical,
format-agnostic representation every parser must produce — the seam that
makes multiple input formats possible without touching extraction or
comparison. It's deliberately flat (no scene/character tree) and has three
`ElementKind`s: `KindDialogue` (attributed to a character), `KindDirection`
(high-confidence: a whole line that's a single parenthesized/bracketed
span, e.g. `(He pauses.)`), and `KindUnclassified` (no positive signal
either way — an honest "don't know," not a guess). A new format parser
needs only to produce a `Script`; nothing else in the pipeline changes.

### Deterministic core

Parsing, normalization, alignment, and diffing (`internal/domain`) are all
plain deterministic Go — dynamic programming (LCS) and string processing,
no LLM calls. This must stay true even after Claude is introduced for note
phrasing: Claude may rephrase or explain a diff the deterministic engine
already computed, but it must never be the thing deciding whether two
lines differ.

### PDF ingestion & AI-assisted parsing

A user can also produce a `domain.Script` by uploading a PDF excerpt
instead of pasting plain text. This is the project's first real external
network dependency (Anthropic) and first real binary-parsing dependency
(PDF), and it slots into the parsing/comparison boundary above as **a
second way to produce a `Script`** — `ExtractCues`/`Align`/`WordDiff` never
change and never see anything before a human has confirmed it.

Three tiers, each less trusted than the last:

1. **Raw extracted text** — `pdftext.ExtractText` (`internal/pdftext`)
   turns uploaded PDF bytes into per-page plain text, or a clear rejection
   (not a PDF, no text layer, too many pages, too much text). It has no
   notion of scripts, characters, or AI.
2. **AI candidate interpretation** — `aiparse.CandidateScript`
   (`internal/aiparse`) is Claude's structured guess at how that raw text
   breaks into dialogue/direction/unclassified elements, reusing
   `domain.ElementKind` but deliberately *not* `domain.ScriptElement` —
   nothing here is trusted yet. Every element carries two independent,
   Go-computed trust signals the model never sets directly: `Page` (which
   page its evidence was actually found on) and `Verified` (see grounding,
   below). Nothing is ever silently dropped at this tier — an unverified
   or unclassified element is still returned, just flagged, so a human can
   decide.
3. **User-confirmed elements** — after review in the browser, the frontend
   sends back only `{kind, character, text}` per surviving row (import-time
   metadata like `Verified`/`Page`/`SourceEvidence` was only ever useful
   *during* review). `internal/api`'s `scriptFromConfirmedElements`
   (`handlers.go`) converts these directly into a `domain.Script` — the
   same type `ParsePlainTextScript` produces — and from there the
   compare request follows exactly one code path (`ExtractCues` → `Align`)
   regardless of which tier produced the script. `compareRequest`'s script
   side is a tagged union (`scriptSourceDTO{Type: "raw"|"elements", ...}`)
   for this reason: two independently-optional fields would make invalid
   states ("both set", "neither set") representable, and this project's
   hand-mirrored TypeScript DTOs make that drift risk real.

**Content grounding** (`aiparse.Verify`, `verify.go`) is what makes an
element's `Verified` flag meaningful, and it's two independent checks, not
one — a single "does Claude's claimed excerpt exist in the raw text" check
is gameable: a real short anchor (e.g. a character's name) could be paired
with a fully invented line and still pass. So `Verify` requires both:
(1) **evidence groundedness** — `SourceEvidence` itself must be found in
some page's raw text, and (2) **text-evidence consistency** — `Text`'s
content words must overlap `SourceEvidence`'s by at least 70%
(`textEvidenceOverlapThreshold`). Both checks run against text normalized
for PDF-extraction artifacts (smart quotes, ligatures, mid-word line-wrap
hyphens) via the same "compare by normalized content words, not exact
bytes" approach `internal/domain` already uses for alignment
(`normalize.go`, `sharesContentWord`) — not a separate scheme invented for
this package.

**Failure policy is deliberately simple, not tuned aggregate scoring**: an
import fails when the response doesn't parse into the expected structure
at all, when it was cut off before finishing
(`aiparse.ErrResponseTruncated`, detected via the API's own `stop_reason`
— see "Error handling" below for why this is its own distinct case), or
when literally zero elements verify (`aiparse.ErrNothingVerified`).
There's no percentage-based aggregate rejection threshold on top of that,
by design — see the known limitations below for why real evaluation data
ended up confirming this rather than motivating a threshold.

The system prompt (`anthropic.go`) also states explicitly that document
content is data to structure, never instructions to follow, since raw PDF
text is user-controlled and adversarial-input-shaped. This is defense in
depth, not the primary defense — grounding is: even a prompt-injected
response still has to survive `Verify` against the real extracted text.

### Saved script library

A user can name and save a script's confirmed elements so they can come
back to it later without re-uploading the PDF or paying for another AI
pass — motivated directly by real cost (a real 5-page import cost about
$0.07) and by the AI interpretation step's non-determinism (see "AI
response reliability" below): the same PDF can come back slightly
differently structured on a second call, so reloading a saved,
already-reviewed result is also the only way to get the *exact same*
structure back, not just an equivalent one.

This is a fourth way to reach the same confirmed-elements shape the
compare pipeline already consumes — not a new code path into
`domain.Script`. `internal/library.Store` persists exactly
`ConfirmedElement`'s shape (`{kind, character, text}`, plus a name and
timestamp) in a local SQLite file (`backend/scripts.db`, gitignored,
matching the existing flat-file-in-`backend/` convention `.env` and
`usage.jsonl` already use). Loading a saved script back still goes
through the *same* `elements` branch of the `scriptSource` tagged union
that a fresh PDF-import confirmation uses — `CompareForm` has never known
or cared where `ConfirmedElement[]` came from, and that stays true here.

Two distinct actions after loading, both without any Anthropic call:
**Load** skips straight to a usable comparison (the content was already
human-reviewed once, so re-reviewing by default would be friction for no
benefit); **Re-review** reopens the exact same editable preview table
used during import, seeded from the saved elements instead of a fresh
`ImportResponse` — the table's `Page`/`Verified` columns get harmless
defaults (`page: 0` → "page unknown", `verified: true`) since those
signals never really applied to reloaded content in the first place (they
meant "Go's grounding check passed against the original PDF," which
didn't run again on a reload) — this is a deliberate repurposing of those
fields for display, not a claim that grounding actually happened.

**Why `library.Store` has no interface**, extending the same
`pdftext`/`aiparse` contrast under "Avoid speculative abstractions"
below: a real `*Store` in tests is local, fast, and deterministic — SQLite
even supports an in-memory database (`:memory:`, or shared-cache mode for
concurrent-access tests) — so there's no slow, costly, non-deterministic
call for a fake to buy speed against. Tests just use a real `*Store`.

## Error handling: distinguish root causes, don't collapse them

When a request can fail for genuinely different reasons, give each one its
own distinct sentinel error and its own actionable message — don't let one
generic error/message stand in for causes that call for a different next
action from the user.

This came from a real bug, not a style preference. `aiparse.InterpretScript`
originally let "the AI's response was cut off by its own output-token
limit before it finished" fall through to the same `ErrNothingVerified`
path as "grounding genuinely rejected every element," so both produced the
identical message: *"the AI's interpretation couldn't be verified against
your document; please try again or paste the script as text."* That's
actively misleading for the truncation case — retrying or pasting plain
text does nothing, because the real fix (a shorter excerpt, or a higher
token limit) is completely different. It wasn't caught until a real
5-page excerpt hit it outside of testing (see the PDF ingestion known
limitations below).

The fix: check the Anthropic API's own `stop_reason` for
`StopReasonMaxTokens` explicitly, before the response is ever handed to
`parseToolInput`/`Verify`, and return a distinct sentinel
(`aiparse.ErrResponseTruncated`) that `internal/api` maps to its own
specific message. Two errors, two messages, because they call for two
different user actions.

Apply this test generally, in any package: before reusing an existing
error or message for a new failure mode, ask whether the user's correct
next action would actually differ (retry vs. shorten input vs. nothing
they can do vs. a config problem only a developer can fix). If it would,
it needs its own distinct error and message, even when the surrounding
code path is otherwise the same.

## Package responsibilities

- `backend/cmd/server` — binary entry point; wires `api.NewMux()` to
  `http.ListenAndServe`, nothing else.
- `backend/internal/domain` — `script.go` (canonical `Script` model +
  `ParsePlainTextScript`), `cues.go` (`Cue` type + `ExtractCues` character
  extraction), `normalize.go` (tokenization), `diff.go` (generic LCS core +
  word-level diff), `align.go` (cue-level diff + changed-pair heuristic).
  No HTTP, no JSON, no external dependencies.
- `backend/internal/pdftext` — `pdftext.go` (`ExtractText`, `PageText`).
  PDF bytes to per-page plain text, or a clear rejection. No notion of
  scripts, characters, or AI; no interface (a single call site can have its
  library swapped without one — that's not what interfaces are for).
- `backend/internal/aiparse` — `aiparse.go` (`CandidateScript`,
  `CandidateElement`, the `ScriptInterpreter` interface), `anthropic.go`
  (the real Anthropic-backed implementation, prompt, and schema),
  `verify.go` (content-grounding validation), `fake.go`
  (`FakeInterpreter`, for fast deterministic tests), `usage.go`
  (per-call token/cost logging — see Commands). Turns raw extracted text
  into a candidate structure; never touches `internal/domain`'s
  comparison types.
- `backend/internal/library` — `library.go` (`Store`, `SavedScript`,
  `Element`, `ErrNotFound`). Persists named, confirmed scripts in a local
  SQLite file. No notion of PDFs, AI, or grounding; no interface (a real
  `*Store` in tests is local, fast, and deterministic — see "Avoid
  speculative abstractions").
- `backend/internal/api` — `handlers.go` (JSON DTOs, `HandleCompare`,
  `NewMux`, the `scriptSource` tagged union, confirmed-elements-to-`Script`
  conversion), `import.go` (`handleScriptImport`, the PDF upload endpoint),
  `library.go` (the saved-script-library endpoints). Translates between
  the wire format and `internal/domain`/`internal/aiparse`/`internal/library`;
  owns validation judgment calls domain functions don't make (e.g.
  rejecting a character absent from the *script* as user error, while
  a character absent from the *transcript* is a valid all-missing result).
- `frontend/src/api` — `types.ts` (hand-written mirror of the Go JSON
  contract — no codegen, so it can drift; check it when `handlers.go`'s
  DTOs change), `compareApi.ts` / `importApi.ts` / `savedScriptsApi.ts`
  (the only places that call `fetch`).
- `frontend/src/features/compare` — the compare form, result rendering
  (`LineNotesList`/`LineNoteRow`/`WordDiffText`), and `useCompare` (async
  request lifecycle).
- `frontend/src/features/import` — `PdfUploadForm.tsx` (file upload),
  `ScriptPreviewTable.tsx` (editable per-row review: relabel kind,
  edit/delete, `Verified: false` flagged), `ScriptImport.tsx` (ties upload
  → preview → confirm together, plus the inline "save to library"
  control), `useImport`/`useSaveScript` (async request lifecycles,
  mirroring `useCompare`).
- `frontend/src/features/library` — `SavedScriptsList.tsx` (browse/Load/
  Re-review/Delete), `useSavedScripts` (fetch-on-mount + `refresh()`).

## Commands

Backend (from `backend/`; `go` may need `/usr/local/go/bin` on `PATH`):
- Run: `go run ./cmd/server` (port 8080) — requires `ANTHROPIC_API_KEY` to
  be set (fails fast at startup otherwise). Local dev keeps it in
  `backend/.env` (gitignored; see `.env.example`), loaded by `main.go`
  relative to the process's working directory — so run this from
  `backend/`, not from inside `cmd/server/`.
- Test: `go test ./...` (add `-v` / `-cover` as needed). This never hits
  the real Anthropic API — `internal/api`/`internal/aiparse` tests use
  `aiparse.FakeInterpreter`.
- Real-API checks (opt-in, not part of the above): `TestAnthropicInterpreterRealSmoke`
  and `TestEvalSuite` in `internal/aiparse` are skipped unless
  `ANTHROPIC_API_KEY` is set in the test's environment. Run explicitly with
  `go test ./internal/aiparse/... -run TestEvalSuite -v` — costs real
  money and has real latency/non-determinism, so it's never run by default.
- Lint/format: `go vet ./...`, `gofmt -l .`
- Build: `go build ./...`
- Check estimated AI spend so far: every real Anthropic call appends one
  JSON line to `usage.jsonl` (gitignored, created on first real call,
  relative to wherever the calling process's working directory is — see
  `usage.go`). Concretely, this means two different files in practice:
  running the real server writes to `backend/usage.jsonl`, but running a
  real-API test via `go test` writes to
  `backend/internal/aiparse/usage.jsonl` instead, since `go test` sets
  its working directory to the package under test, not `backend/`. Check
  whichever one is relevant to what you just ran. Total estimated cost:
  `jq -s 'map(.estimated_cost_usd) | add' usage.jsonl`. This is an
  estimate against list price, not a bill — the Anthropic Console's own
  usage dashboard is the authoritative source for actual spend; this file
  exists to correlate cost with specific app-level calls, which the
  Console can't do.
- Saved scripts live in `backend/scripts.db` (gitignored, a real SQLite
  file — created on first save, relative to the process's working
  directory, same convention as `.env`/`usage.jsonl`). Inspect it directly
  with `sqlite3 scripts.db "select id, name, created_at from saved_scripts"`
  if the `sqlite3` CLI is available, or just use the app's own
  `GET /api/scripts/saved` endpoint.

Frontend (from `frontend/`):
- Run: `npm run dev` (proxies `/api` to `localhost:8080`, see
  `vite.config.ts`)
- Lint: `npm run lint`
- Type-check: `npx tsc -b`
- Build: `npm run build`

## Testing expectations

- `internal/domain` tests are black-box (`package domain_test`) against
  `ParsePlainTextScript`/`ExtractCues`/`WordDiff`/`Align`. The one exception
  is `lcsDiff` itself
  (`diff_internal_test.go`, white-box `package domain`) — it's tested
  directly with plain generic inputs because degenerate DP edge cases
  (empty slices, fully disjoint sequences) don't map onto realistic cue
  fixtures.
- New domain logic needs unit tests for the behaviors that actually
  require alignment (not just positional zipping): a single dropped/added
  cue, consecutive drops/adds, a paraphrase, and cases that exercise the
  pairing heuristic's token-sharing gate.
- `internal/api` tests are plumbing-only (status codes, JSON shape) via
  `httptest` — don't re-test domain edge cases at this layer.

## Avoid speculative abstractions

No interfaces, provider abstractions, or repository patterns until a
second concrete implementation actually needs one. An interface is
justified when it marks a real external-service or testing boundary
(e.g. a future Anthropic client the API layer depends on) — that's a
legitimate seam, not speculation. `ParsePlainTextScript`, `ExtractCues`,
and `Align` have neither an external dependency nor a second
implementation, so they stay plain functions. In particular: no `Parser`
interface until a second concrete parser (Fountain, PDF, ...) actually
exists — introduce it then, keyed off that second implementation, not in
anticipation of one. Same for parse warnings/diagnostics: there's no
consumer for them yet (no parse-preview UI, no diagnostics field in the
API contract), so don't add a warnings type speculatively; `Script`
already preserves the relevant information passively via
`KindUnclassified`.

`aiparse.ScriptInterpreter` is the one interface this codebase actually
has, and it earns its keep for a different reason than "might need a
second implementation someday": a real implementation means a real
network call with real cost, latency, and non-determinism, so a fast,
deterministic fake (`aiparse.FakeInterpreter`) buys test speed a real call
never could — that's the legitimate seam, not swappability. Contrast with
`pdftext.ExtractText`, which has no interface despite also wrapping a
third-party library: it's local, fast, and deterministic, so its one call
site can have its library swapped with zero interface needed. The
lesson generalizes: an interface is for a *testing or deployment* boundary
that actually needs one, not for "this wraps an external package."

`internal/library.Store` is a second worked example on the no-interface
side, and a different kind of dependency than `pdftext`'s (a real
database, not a parsing library) — showing this isn't just about
"parsing libraries don't need interfaces," it's about the call itself
being local, fast, and deterministic regardless of what the dependency
is. A real `*Store` in tests is exactly that (SQLite even offers an
in-memory mode for zero filesystem I/O), so tests use one directly
instead of a fake.

## Known limitation: the cue-pairing heuristic in `align.go`

`alignGap` pairs a gap's deletes and inserts *positionally* (first with
first, second with second, ...), gated by "do the two cues share at least
one normalized *content* word" (`sharesContentWord`, `align.go`) — a small
hardcoded stop-word list (articles, pronouns, prepositions, conjunctions,
forms of "be"/"do"/"have") is excluded from that check specifically so two
unrelated cues that both happen to contain "the" or "is" aren't mistaken
for a paraphrase of each other. This is still a cheap heuristic, not a
similarity ranking, and it is known to mis-pair when two genuinely
unrelated cues happen to share a real content word by coincidence (e.g.
both mention the same proper noun or topical word) — that pair is still
reported as a false `changed` note instead of separate `missing`/`extra`
notes (pinned by `TestAlignUnrelatedCuesSharingOneContentWordAreFalselyPairedAsChanged`).
It's also known to under-pair a heavy paraphrase that shares zero
vocabulary at all (rare), or a substitution where every *other* word in the
cue is a stop word (e.g. "I love you" → "I hate you" has no shared content
word once "I"/"you" are excluded) — both get reported as `missing`+`extra`
instead of `changed` (pinned by
`TestAlignShortSubstitutionWithOnlyStopWordsInCommonIsFalselyMissingExtra`).

**This is an accepted deterministic baseline, not a bug to keep chasing.**
Both pinned tests assert today's behavior *as a known limitation*, not as
desired behavior — their names and comments say so explicitly. Don't add
further tuning to the word-overlap gate (weighting, thresholds, synonym
lists, etc.) without a concrete case motivating it; the two tests exist so
that if the gate is ever changed, updating them is a deliberate, visible
decision, not a silently-passing accident.

**Future semantic evaluation must narrow, not replace, this pipeline.** If
Claude is ever introduced to reduce these false positives/negatives (per
the "Deterministic core" principle above — Claude phrases or explains, it
never decides whether two lines differ), it must operate only on the
*ambiguous candidate pairs* deterministic alignment already produced —
e.g. "is this positionally-adjacent missing+extra pair actually a
paraphrase?" — not run over raw cue sequences as a second, LLM-driven
alignment algorithm. `Align` always runs first and produces the full
candidate structure (`exact`/`changed`/`missing`/`extra`); a semantic step,
if added, relabels or explains specific pairs within that structure and
nothing upstream of it.

## Known limitations: plain-text parsing in `script.go`

`ParsePlainTextScript` only recognizes the `CHARACTER:` colon convention,
where `CHARACTER` contains no lowercase letters. Specifically, and treated
as accepted, stated limitations rather than bugs to silently work around:

- No centered/standalone character-name lines (name alone on one line,
  dialogue starting the next) — a distinct convention with its own design
  questions, not yet supported.
- A source label must be all-caps to be recognized as starting a cue (this
  replaced a plain "any colon starts a cue" rule specifically to resolve
  the ambiguity of dialogue that itself contains a colon, e.g. "She said:
  come here" — see `isNewCueLine`'s doc comment). Matching a *requested*
  character name at extraction time is still case-insensitive; only the
  script's own authored label must be capitalized.
- A blank line always ends a continuation, even for a mid-speech dramatic
  pause a human would read as one continuous cue.
- An all-caps phrase immediately before a colon inside continued dialogue
  (e.g. "TO ARMS: everyone move now" as a spoken line) is still misread as
  a new cue.
- Consecutive stage-direction lines are not merged into one element; each
  bracketed/parenthesized line is its own `KindDirection` element.

No general inconsistent-formatting tolerance is attempted. Revisit any of
these only when real usage demonstrates they matter.

## Known limitation: `WordDiffSpan` only carries one surface string

`WordDiff`'s `"equal"` spans store the surface text from the *script* side
only (`diff.go`), since a real diff's equal words are normally assumed
byte-identical. That assumption breaks when two cues are normalized-equal
but surface-different (e.g. `"Wait: ..."` vs `"Wait, ..."`) — the spoken
side's own punctuation/wording for an equal word isn't recoverable from
`Diff` alone. For `exact` notes this doesn't matter: cue-level equality
guarantees the whole `Diff` is `"equal"` spans, so the frontend
(`LineNoteRow.tsx`) renders `scriptText`/`spokenText` directly instead of
going through `WordDiffText`. The narrower case is `changed` notes that mix
real changes with equal-but-surface-different words in the same cue — those
still show the script's surface form for the equal portions on the spoken
side. Fixing that fully would mean `WordDiffSpan` carrying separate
script/spoken text for equal spans; not done since it's not the reported
case and touches the wire contract. Revisit if real usage shows it matters.

## AI response reliability: known failure modes and how they're handled

Every call to Claude in this project carries two distinct kinds of risk:
the API call itself can fail or behave unexpectedly (truncation, malformed
tool output, refusal), and even a *successful* call's judgment can be
imperfect or inconsistent (misclassification, uneven cleanup). These are
different problems needing different treatment — the first category is
about how we handle the call, the second is inherent to using an LLM for
structural judgment at all and can only be mitigated, never eliminated in
code. This section catalogs both, and for each: is it currently caught
and given a clear, distinct error, is it a known gap worth fixing, or is
it inherent and handled architecturally instead.

- **Response truncated mid-generation (`stop_reason: max_tokens`) — fixed,
  caught explicitly.** A real 5-page excerpt hit this in practice:
  `maxResponseTokens` (`anthropic.go`) was sized for the single-page
  smoke-test fixture (4096), far too small for a real excerpt's full
  structured output, so the response was silently cut off and
  misreported as a grounding failure. Fixed by raising the limit to
  16000 (confirmed against the real excerpt) and by checking the API's
  own `stop_reason` explicitly, before the response is ever parsed,
  returning a distinct `aiparse.ErrResponseTruncated` with its own
  message (see "Error handling" above). Accepted residual risk: an
  unusually dense excerpt right at this slice's own caps (15 pages/20K
  chars) could still exceed 16000 tokens of structured output — not
  eliminated, but now caught and communicated clearly rather than
  misattributed to grounding. Raising the limit further would cross into
  Anthropic's streaming-required threshold (confirmed empirically: 32000
  triggered "streaming is required for operations that may take longer
  than 10 minutes"), which isn't worth the added implementation
  complexity until this slice actually needs longer documents than it
  supports today.
- **Tool input double-encoded as a JSON string — known gap, not yet
  fixed.** Observed once during manual testing, non-deterministically
  (the same excerpt succeeded normally on other calls): Claude
  occasionally wraps its entire answer as an escaped JSON *string*
  instead of matching the schema's literal array type for `elements` — a
  known category of forced-tool-use quirk on large structured outputs.
  Today this fails as a raw `json.Unmarshal` type error inside
  `parseToolInput`, surfaced only via the generic "AI parsing is
  temporarily unavailable; please try again" message
  (`internal/api/import.go`), not something specific. This is
  improvable: `parseToolInput` could detect that shape (the decoded
  value is a string, not an object) and recursively parse it as a
  fallback before giving up. Not yet done — flagged here as the next
  concrete thing worth fixing in this area.
- **Model refusal (`stop_reason: refusal`) — known gap, not yet
  handled.** The Anthropic SDK defines this stop reason, but
  `InterpretScript` doesn't check for it distinctly; today it would fall
  through to the generic "response did not include a
  submit_script_structure tool call" error. Not observed in practice
  (public-domain/theatrical script content isn't typically
  refusal-triggering) and not added preemptively — consistent with this
  project's avoid-speculative-abstractions stance, this is worth adding
  explicit handling for if it's ever actually seen, not before.
- **Misclassification (`kind`/`character` judgment) — inherent, not
  fixable in code.** `Verify` confirms `SourceEvidence` is real text from
  the document and that `Text` is substantially consistent with it — it
  says nothing about whether Claude's structural judgment (is this
  dialogue or direction? whose line is it?) was *correct*. There's no
  independent ground truth for "is this classification right" the way
  there is for "does this text exist in the document," so this can't be
  validated away. The only correct mitigation is the human review/edit
  step before confirmation — that's by design, not a gap to close.
- **Inconsistent cleanup across equivalent content — inherent, same
  mitigation.** Manual testing found the same document had an inline
  parenthetical aside stripped from one dialogue line's `Text` but left
  attached to another, with nothing distinguishing the two that would
  explain the different treatment. Same root cause as misclassification:
  a property of the model's own non-deterministic judgment, not a bug in
  our code — mitigated the same way, by review before use, never treated
  as a correctness signal to chase.
- **Aggregate output quality varies call to call — monitored with real
  data, not tuned preemptively.** There's no percentage-based aggregate
  rejection threshold beyond "zero elements verified"
  (`aiparse.ErrNothingVerified`) by design. Stage E's real-API evaluation
  suite (`aiparse.TestEvalSuite`) checked this with real data: every real
  fixture came back 100% verified, with no observed "mostly good, some
  noise" case to motivate a threshold. This could change with more
  real-world usage; revisit only with concrete contrary evidence, not
  preemptively.

None of this is correctness-critical, which is the point of the
pipeline's shape: Claude never decides final content (the "Deterministic
core" principle above), `Verify` independently checks its claims against
the source, and a human confirms every row before anything downstream
ever sees it. Improving reliability further is about reducing *noise* —
fewer confusing error messages, fewer rows wrongly flagged unverified —
not about making the system safe to trust blindly, which it was never
designed to be.

## Known limitations: PDF ingestion

- **Short excerpts only, no chunking.** `pdftext.MaxPages`/
  `MaxExtractedChars` cap this to a scene or short excerpt, not a full
  script. A short excerpt fits one structured-output call; whole-play
  import would need a long-document/chunking strategy that hasn't been
  designed. Rejected clearly at upload time, not silently truncated.
- **No OCR.** A PDF with no embedded text layer (a scanned page rendered
  as an image) is rejected (`pdftext.ErrNoTextLayer`) rather than attempted
  — out of scope for this slice.
- **A PDF's own font encoding can silently drop letters, unrelated to
  anything AI or grounding-related.** Found via manual testing: two pages
  of a real excerpt (a title page and cast list, styled with a decorative
  font) extracted with specific letters missing entirely — e.g. "Theseus"
  came out as " eseus," "Titania" as "Ti ania" — because that font's
  embedded character encoding had no mapping for certain glyphs. This is
  a property of how the source PDF was authored/exported, not something
  `pdftext` or any code here can detect or repair; elements sourced from
  such a page correctly fail verification, since the extracted text
  genuinely doesn't match those words. Nothing to fix — just something to
  expect from real, especially decoratively-formatted, source PDFs.
- **The word-overlap check is a bag-of-words check, not an ordering
  check** (`overlapRatio`, `verify.go`) — text whose words are a real
  anagram of its evidence's would pass. Same category of accepted,
  narrow heuristic limitation as `align.go`'s pairing gate above; not
  tightened without a concrete case motivating it.
- **An *in-progress, unsaved* review still isn't persisted across a page
  refresh.** Candidate elements only exist in React state until either
  confirmed (handed to the compare form) or explicitly saved to the
  library; reloading mid-review loses that work either way. Flagged in
  the UI, not fixed. This is narrower than it used to be: a *saved*
  script (see "Saved script library" above) does survive a refresh or a
  full server restart — it's specifically the unsaved, mid-review state
  that has no persistence, not the app as a whole anymore.
- **`Page` is where evidence *starts*, not necessarily where it's fully
  contained.** `findEvidencePage` (`verify.go`) searches the whole
  document's text as one continuous string specifically so a line or
  speech spanning a physical page break can still verify — found via
  manual testing with a real script, where a Hermia speech split across
  two pages was wrongly reported as unverified before this fix. One
  related, narrower artifact this required handling: PDF exports often
  leave a printed page-footer number as the last extracted token on a
  page, which would otherwise wedge itself between two pages' real
  content at the exact join point; `stripTrailingPageNumber` strips a
  short, standalone trailing digit run per page before joining, for
  cross-page search only. Still-accepted, narrower limitation: a
  hyphenated word split exactly at a page boundary (not a mid-page
  line-wrap, which already rejoins) won't be rejoined, since page-number
  attribution requires normalizing each page separately before joining.
