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
  (`FakeInterpreter`, for fast deterministic tests). Turns raw extracted
  text into a candidate structure; never touches `internal/domain`'s
  comparison types.
- `backend/internal/api` — `handlers.go` (JSON DTOs, `HandleCompare`,
  `NewMux`, the `scriptSource` tagged union, confirmed-elements-to-`Script`
  conversion), `import.go` (`handleScriptImport`, the PDF upload endpoint).
  Translates between the wire format and `internal/domain`/`internal/aiparse`;
  owns validation judgment calls domain functions don't make (e.g.
  rejecting a character absent from the *script* as user error, while
  a character absent from the *transcript* is a valid all-missing result).
- `frontend/src/api` — `types.ts` (hand-written mirror of the Go JSON
  contract — no codegen, so it can drift; check it when `handlers.go`'s
  DTOs change), `compareApi.ts` / `importApi.ts` (the only places that call
  `fetch`).
- `frontend/src/features/compare` — the compare form, result rendering
  (`LineNotesList`/`LineNoteRow`/`WordDiffText`), and `useCompare` (async
  request lifecycle).
- `frontend/src/features/import` — `PdfUploadForm.tsx` (file upload),
  `ScriptPreviewTable.tsx` (editable per-row review: relabel kind,
  edit/delete, `Verified: false` flagged), `ScriptImport.tsx` (ties upload
  → preview → confirm together), `useImport` (async request lifecycle,
  mirroring `useCompare`).

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

## Known limitations: PDF ingestion

- **Short excerpts only, no chunking.** `pdftext.MaxPages`/
  `MaxExtractedChars` cap this to a scene or short excerpt, not a full
  script. A short excerpt fits one structured-output call; whole-play
  import would need a long-document/chunking strategy that hasn't been
  designed. Rejected clearly at upload time, not silently truncated.
- **No OCR.** A PDF with no embedded text layer (a scanned page rendered
  as an image) is rejected (`pdftext.ErrNoTextLayer`) rather than attempted
  — out of scope for this slice.
- **Grounding proves evidence existed and is consistent, not that
  classification is correct.** `Verify` confirms `SourceEvidence` is real
  text from the document and that `Text` is substantially the same words —
  it says nothing about whether Claude's `kind`/`character` judgment was
  right (e.g. calling a stage direction dialogue, or misattributing a
  line). That's what the human review step is for, not validation.
- **The word-overlap check is a bag-of-words check, not an ordering
  check** (`overlapRatio`, `verify.go`) — text whose words are a real
  anagram of its evidence's would pass. Same category of accepted,
  narrow heuristic limitation as `align.go`'s pairing gate above; not
  tightened without a concrete case motivating it.
- **No aggregate rejection threshold, and Stage E's real-API evaluation
  suite (`aiparse.TestEvalSuite`) confirmed this rather than motivating
  one to be added**: every real fixture came back 100% verified, with no
  observed "mostly good, some noise" case for a percentage threshold to
  actually help with. The only aggregate failure condition remains
  `aiparse.ErrNothingVerified` (literally zero elements verified).
- **Claude's own cleanup is inconsistent, not just imperfect
  extraction.** Manual testing found the same document had an inline
  parenthetical aside stripped from one dialogue line's `Text` but left
  attached to another, with nothing distinguishing the two that would
  explain the different treatment — a real behavior quirk to expect in
  review, not a bug to chase.
- **No persistence across a page refresh during import review.** Confirmed
  elements only exist in React state until handed to the compare form;
  reloading loses an in-progress review. Flagged in the UI, not fixed —
  there's no persistence layer anywhere else in this project either.
