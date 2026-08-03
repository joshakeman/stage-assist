---
name: eval-runner
description: Runs Stage Assist's real-API PDF-parsing evaluation suite (aiparse.TestEvalSuite) and reports a clean pass/fail summary with cost impact, instead of leaving raw go test -v output to be parsed by hand.
---

# Eval runner

Wraps `aiparse.TestEvalSuite` (`backend/internal/aiparse/eval_test.go`) —
the opt-in, real-Anthropic-API evaluation suite for this project's
AI-assisted PDF import. It exists to catch regressions in Claude's
real-world PDF-parsing behavior (a prompt tweak, a model change, a code
change) that fast, mocked tests can't detect, since those never touch
the real API at all.

**This costs real money and calls the real Anthropic API.** Only run it
when the user explicitly invokes this skill or clearly asks for the eval
suite to be run — never proactively, and never as part of routine
`go test ./...` work.

## What it checks

Three known PDF fixtures (`backend/internal/pdftext/testdata/`), each a
different layout the deterministic plain-text parser can't handle on its
own: a plain `CHARACTER:` colon-labeled baseline, a centered-character-name
layout, and an inline-parenthetical monologue. For each, the real API is
called and the result is checked against known-good minimums recorded
from earlier real runs (at least one verified element, and a minimum
dialogue-line count per named character) — see the `evalCases` table at
the top of `eval_test.go` for the exact fixtures and thresholds in effect
today; don't hardcode this list here, since it can change independently
of this skill.

## Steps

1. **Confirm an API key is available.** Check the shell environment for
   `ANTHROPIC_API_KEY`. If it's not set there, read it from
   `backend/.env` (gitignored) the same way `main.go` does. If it's
   missing from both, stop and tell the user to set it (see
   `backend/.env.example`) — do not proceed without it.
2. **Record the cost baseline.** `usage.jsonl` is written relative to the
   *process's* working directory (see `aiparse/usage.go`), and `go test`
   runs with that working directory set to the package under test — so
   running this suite writes to `backend/internal/aiparse/usage.jsonl`,
   **not** `backend/usage.jsonl` (that path only fills up when the real
   server binary itself makes a call, from `backend/`). Note the current
   line count of `backend/internal/aiparse/usage.jsonl` before running
   (treat it as 0 lines if the file doesn't exist yet), so the cost of
   *this* run can be reported afterward rather than the cumulative total.
3. **Run the suite** from `backend/`, with the API key exported into the
   shell environment:
   ```
   go test ./internal/aiparse/... -run TestEvalSuite -v
   ```
4. **Parse the output.** For each subtest
   (`--- PASS: TestEvalSuite/<name>` / `--- FAIL: TestEvalSuite/<name>`):
   record pass or fail, pull the `t.Logf` line just above it
   (`elements=N verified=N dialogue-counts=map[...]`) for the actual
   observed counts, and for any failure, pull the specific `t.Errorf`
   message(s) explaining which threshold wasn't met and by how much.
5. **Report the cost impact.** Compare `backend/internal/aiparse/usage.jsonl`'s
   new line count against the baseline from step 2, and sum
   `estimated_cost_usd` for just the new lines, so the user sees exactly
   what this run cost — not the all-time total (that's what
   `jq -s 'map(.estimated_cost_usd) | add' usage.jsonl` is for, per
   `CLAUDE.md`'s Commands section, run against whichever `usage.jsonl`
   is relevant).
6. **Present a summary**, not raw test output: one line per fixture
   (pass/fail, element/verified/dialogue counts, failure reason if any),
   then totals — how many fixtures passed out of how many, and this
   run's estimated cost.
7. **If anything failed, say so plainly and ask before moving on.** A
   failure here means real-world AI parsing behavior has drifted from
   what the fixture thresholds expect — treat it as worth investigating,
   not something to note in passing and continue past.

## Notes

- This suite is deliberately excluded from `go test ./...` and from any
  CI — see `CLAUDE.md`'s "AI response reliability" section for why (real
  cost, real latency, real non-determinism).
- The thresholds in `evalCases` are calibrated against real prior runs,
  not guessed. If a failure here turns out to reflect a real, permanent
  behavior change rather than a fluke, updating those thresholds is a
  deliberate edit for the user to make in `eval_test.go` — this skill
  reports the mismatch, it doesn't silently paper over it by adjusting
  numbers itself.
