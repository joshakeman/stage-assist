# Stage Assist

Stage Assist helps an actor find out where their line delivery drifted
from the script. Give it the original scene and a transcript of a
rehearsal (or a run-through, a recording transcription, whatever you've
got), and it lines up the two, cue by cue, and shows you exactly what
changed: a word swapped here, a line dropped there, a whole speech added
that wasn't in the original.

It's also a personal project for practicing idiomatic Go and React/
TypeScript, so alongside "does it work," a fair amount of attention here
has gone into "is it built well" — clear boundaries between pieces,
deliberate and documented tradeoffs, and tests that pin down behavior
rather than just checking the happy path.

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
is what let PDF import get added later as an entirely new *front door* —
a second way to produce the same internal script representation — without
touching a single line of the comparison logic itself.

For the full technical breakdown (package layout, the reasoning behind
specific design decisions, exact known limitations, and the conventions
this repo follows), see [`CLAUDE.md`](./CLAUDE.md). This document stays
at the "what and why," not the "how."

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
  *can* handle that layout). Fountain, DOCX, and other formats aren't
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
- **No persistence.** Nothing is saved anywhere — not a script, not a
  transcript, not a completed comparison, not an in-progress PDF import.
  Refresh the page and it's gone. There's no login, no accounts, no
  database.
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
- **Persistence and accounts.** Saving scripts, transcripts, and past
  comparisons across sessions — the biggest architectural addition on
  this list, since nothing like it exists yet.
- **Claude-authored note phrasing.** Once a diff is computed, a short
  natural-language note ("you dropped the second half of this line") could
  be more useful to an actor than a raw word-level diff — again, narrating
  a result the deterministic engine already produced, not replacing it.
