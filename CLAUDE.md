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

## Package responsibilities

- `backend/cmd/server` — binary entry point; wires `api.NewMux()` to
  `http.ListenAndServe`, nothing else.
- `backend/internal/domain` — `script.go` (canonical `Script` model +
  `ParsePlainTextScript`), `cues.go` (`Cue` type + `ExtractCues` character
  extraction), `normalize.go` (tokenization), `diff.go` (generic LCS core +
  word-level diff), `align.go` (cue-level diff + changed-pair heuristic).
  No HTTP, no JSON, no external dependencies.
- `backend/internal/api` — `handlers.go` (JSON DTOs, `HandleCompare`,
  `NewMux`). Translates between the wire format and `internal/domain`;
  owns validation judgment calls domain functions don't make (e.g.
  rejecting a character absent from the *script* as user error, while
  a character absent from the *transcript* is a valid all-missing result).
- `frontend/src/api` — `types.ts` (hand-written mirror of the Go JSON
  contract — no codegen, so it can drift; check it when `handlers.go`'s
  DTOs change), `compareApi.ts` (the one place that calls `fetch`).
- `frontend/src/features/compare` — the compare form, result rendering
  (`LineNotesList`/`LineNoteRow`/`WordDiffText`), and `useCompare` (async
  request lifecycle).

## Commands

Backend (from `backend/`; `go` may need `/usr/local/go/bin` on `PATH`):
- Run: `go run ./cmd/server` (port 8080)
- Test: `go test ./...` (add `-v` / `-cover` as needed)
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
