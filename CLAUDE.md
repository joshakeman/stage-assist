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

`ParseCues` (`internal/domain/cues.go`) is the only place that knows about
"character" — it extracts one speaker's ordered lines from raw script/
transcript text. Everything downstream (`Align`, `WordDiff`) operates on
plain ordered `Cue`/`Token` sequences with no notion of character or
speaker. Keep it this way: comparison logic must stay agnostic to how its
input sequences were selected, so supporting a different extraction (e.g.
a whole scene) is a change to `cues.go` only.

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
- `backend/internal/domain` — `cues.go` (character extraction),
  `normalize.go` (tokenization), `diff.go` (generic LCS core + word-level
  diff), `align.go` (cue-level diff + changed-pair heuristic). No HTTP,
  no JSON, no external dependencies.
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
  `ParseCues`/`WordDiff`/`Align`. The one exception is `lcsDiff` itself
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
legitimate seam, not speculation. `ParseCues` and `Align` have neither
an external dependency nor a second implementation, so they stay plain
functions.

## Known limitation: the cue-pairing heuristic in `align.go`

`alignGap` pairs a gap's deletes and inserts *positionally* (first with
first, second with second, ...), gated only by "do the two cues share at
least one normalized word." This is a cheap heuristic, not a similarity
ranking, and it is known to mis-pair in real usage — e.g. a genuinely
dropped cue and an unrelated added cue that happen to share a common
stopword (like "the") get reported as one false `changed` note instead of
separate `missing`/`extra` notes. Treat this as a provisional
implementation: don't build new features that assume it's exact, and if
it's revisited, prefer improving the similarity gate over adding a
different alignment algorithm on top of it.
