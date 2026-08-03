# Stage Assist

Stage Assist helps an actor find out where their line delivery drifted
from the script. Give it the original scene and a transcript of a
rehearsal (or a run-through, a recording transcription, whatever you've
got), and it lines up the two, cue by cue, and shows you exactly what
changed: a word swapped here, a line dropped there, a whole speech added
that wasn't in the original.

## What it does

1. **Bring a script.** Either paste it as plain text (using a simple
   `CHARACTER: dialogue` convention), or upload a short PDF excerpt and
   let Claude read the structure out of it for you.
2. **Bring a transcript.** Plain text, same convention — this is what
   actually got said, however it was captured.
3. **Pick a character.** Stage Assist pulls just that character's lines
   out of both documents, in order.
4. **Get line notes.** Each of that character's lines gets a status —
   exact, changed, missing, or extra — with a word-level diff on anything
   that changed, so you can see precisely which words were off.

The comparison itself is entirely deterministic: no AI is involved in
deciding whether two lines match or how they differ. That's a plain,
well-tested string-alignment algorithm. AI only comes in earlier, as an
optional way to turn a PDF into structured text in the first place — and
even then, nothing it produces is trusted until you've reviewed it.

## How it's put together

A Go backend (`backend/`, plain standard library, no web framework) does
all the real work; a React/TypeScript frontend (`frontend/`) is a thin
client that only ever talks to that backend — it never calls any AI
service directly, and no AI credentials ever ship to the browser.

Internally, turning raw text into line notes is a pipeline where each
stage only knows about the stage immediately before it: parsing a script
format has no idea what a "character" is, extracting one character's
lines has no idea what format the script came from, and comparing two
sequences of lines has no idea any of that ever happened. That separation
is what let PDF import get added later as an entirely new _front door_ —
a second way to produce the same internal script representation — without
touching a single line of the comparison logic itself.

For the full technical breakdown (package layout, the reasoning behind
specific design decisions, exact known limitations, and the conventions
this repo follows), see [`CLAUDE.md`](./CLAUDE.md). This document stays
at the "what and why," not the "how."

## How this was built with Claude

Every feature here — the deterministic comparison engine, the PDF import
pipeline, the saved-script library — was built in collaboration with
Claude Code, and the process is as much a part of this project as the
code is. A few concrete moments that show what that collaboration
actually looked like, rather than just asserting it happened:

- **A security gap caught before it became code.** The original design
  for verifying that Claude's PDF interpretation wasn't fabricated only
  checked "does this excerpt exist somewhere in the source text." A
  plan-review pass caught that this was gameable — a real, trivial anchor
  (a character's name) could be paired with entirely invented dialogue
  and still pass. The fix (requiring the delivered text to also overlap
  the anchor, not just exist near it) was designed and tested before a
  single line of the original approach was ever written.
- **Real API calls, not just fakes, at the stages that mattered.**
  Anthropic's API is genuinely non-deterministic, so a mocked test
  proving an AI-dependent feature "works" would be proving the wrong
  thing. The structured-output schema was validated against one real
  call before any of the surrounding validation or UI was built around
  it, and a whole evaluation suite exists specifically to check real
  script layouts against the real API — deliberately gated so it never
  runs by accident, since it costs real money.
- **Two real bugs, found by actually using the app, not just testing
  it.** A real 5-page script import returned a confusing "couldn't be
  verified" error; digging in found that Claude's response was silently
  getting cut off by a token limit sized for a much smaller test
  fixture — fixed, and confirmed by deliberately forcing the same
  failure again to see the new, clearer error message this time.
  Separately, a line of dialogue that genuinely spans a printed page
  break was wrongly flagged unverified, which led to a second, subtler
  bug underneath it (a page-footer number a PDF-extraction library was
  reading as part of the dialogue) that would have kept the first fix
  from working at all on that real document.
- **Decisions made and written down, not just made.** When it became
  clear that avoiding all persistence was costing real money on every
  repeat use, that tradeoff got discussed explicitly, decided on, and
  then reflected honestly in this project's own documentation — rather
  than left to quietly drift out of date the moment reality changed.

None of this reflects "AI writes the code, human reviews it" so much as
an ongoing back-and-forth: proposing a design for critique, catching
cases where a plausible-looking approach had a real hole in it, and
treating "the AI says it works" as a claim to verify against a real
system, not a fact to accept. The fullest record of that lives in
[`CLAUDE.md`](./CLAUDE.md) — the architecture brief that kept a long,
multi-session collaboration consistent, updated at every stage rather
than written once and left behind.

## Getting started

You'll need Go and Node installed. From the repo root:

```bash
# Backend (port 8080)
cd backend
cp .env.example .env   # then fill in a real ANTHROPIC_API_KEY
go run ./cmd/server

# Frontend (in a second terminal)
cd frontend
npm install
npm run dev
```

The `ANTHROPIC_API_KEY` is only required for PDF import — pasting plain
text works without it, but the server currently fails fast at startup if
the key is missing at all, so you'll need a key (even a placeholder won't
do; it has to be a real one to actually import a PDF).

## What it's good at, and what it isn't (yet)

**Solid today:**

- Comparing two plain-text scripts and getting a reliable, explainable
  diff, including handling dropped lines, added lines, and paraphrased
  lines, not just a naive line-by-line zip.
- Importing a short PDF excerpt (a scene, not a full play) into that same
  comparison flow, including layouts a simple text parser couldn't handle
  on its own — centered character names, inline stage directions mid-line,
  and so on — because an AI is doing the structural reading, with its
  output checked against the source before you ever see it.
- A review step before anything AI-produced is used: every imported line
  is editable, deletable, and flagged if it couldn't be confirmed against
  the source document, so nothing gets into a comparison without a human
  saying so.
- A named library for scripts you've already imported and reviewed: save
  one once, and come back to it later without re-uploading the PDF or
  paying for another AI pass — a real, non-trivial saving, since each
  real import costs a small but real amount in AI usage. Loading skips
  straight to a usable comparison; re-opening the same familiar review
  table for a second look never re-calls the AI either, since the
  reviewed content is already saved locally.

**Known gaps, by design (not oversights):**

- **Short excerpts only.** PDF import is capped at a handful of pages.
  Whole-play import would need a genuinely different strategy (splitting
  a long document across multiple AI calls and stitching the result back
  together), which hasn't been built.
- **No scanned PDFs.** If a PDF has no real text layer — it's just a
  picture of a page — it's rejected outright. No OCR is attempted.
- **One plain-text script format today.** Pasted text only understands
  the `CHARACTER:` colon convention; a name centered alone on its own line
  isn't recognized as starting a new cue (though the AI-assisted PDF path
  _can_ handle that layout). Fountain, DOCX, and other formats aren't
  supported yet, but the architecture was specifically designed so adding
  a new one is a contained addition, not a rewrite of the comparison
  logic.
- **The line-pairing heuristic is intentionally simple.** When lines are
  dropped in one place and added in another nearby, Stage Assist has to
  guess whether that's really one paraphrased line or two unrelated ones.
  It uses a cheap, fast heuristic (shared vocabulary) rather than a full
  similarity model, and it has known, deliberately-accepted edge cases
  where it guesses wrong. A future semantic pass (using Claude to
  adjudicate just the ambiguous cases) is a plausible next step, but it
  would only ever narrow this heuristic's blind spots, never replace the
  deterministic comparison itself.
- **Persistence is scoped to named, confirmed scripts only.** Saving a
  reviewed script to the library is the one thing that survives a refresh
  or even a server restart. Everything else still doesn't: a transcript,
  a completed comparison, and an _in-progress, unsaved_ PDF review are
  all gone the moment you reload the page. There's no login, no
  accounts, and no support for more than one person's library — it's a
  single local file, not a real multi-user backend.
- **No deployment story.** This runs locally, for now. Taking it further
  (auth, storage, hosting) is future scope, not something this project
  has attempted.

## Where this could go next

A few directions that would be natural extensions of what's here, roughly
in order of how well they fit the existing architecture:

- **Semantic line-pairing.** Use Claude to resolve the ambiguous cases the
  deterministic pairing heuristic gets wrong today — always as a second
  opinion on candidates the deterministic pass already found, never as a
  replacement for it.
- **More script formats.** Fountain (a plain-text screenwriting standard)
  would be the most natural second format to support, since it's still
  plain text with its own conventions.
- **Whole-script PDF import.** Chunking a longer document across multiple
  AI calls and reassembling the result, building on the same review-
  before-trust pattern already in place for short excerpts.
- **Accounts and a real multi-user backend.** A first, deliberately small
  slice of persistence already exists (a local, named script library —
  see above), but it's a single local file with no login and no concept
  of separate users. Saving transcripts and past comparison results, and
  supporting more than one person's library, remain the real unbuilt
  next step here.
- **Claude-authored note phrasing.** Once a diff is computed, a short
  natural-language note ("you dropped the second half of this line") could
  be more useful to an actor than a raw word-level diff — again, narrating
  a result the deterministic engine already produced, not replacing it.
