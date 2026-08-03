# Comparison feature — manual testing issues

Running log of issues found while manually testing the compare feature
against real scripts/transcripts. Add a new entry per issue as you find
it; we'll go through them together afterward.

Not everything here will turn out to be a bug — the alignment heuristic
in `align.go` has some known, accepted limitations (documented in
`CLAUDE.md`) around mis-pairing paraphrases and stop-word-only
substitutions. Flag anything that looks wrong regardless; sorting
"known limitation" from "genuine bug" is part of what we'll do together.

---

## Template (copy this for each new issue)

### Issue N: <short title>

- **Character:** 
- **Script line(s):**
  ```
  
  ```
- **Transcript line(s):**
  ```
  
  ```
- **What Stage Assist showed:** (status per line — exact/changed/missing/extra — and the diff if relevant)
- **What I expected instead:**
- **Notes:** (optional — why you think it's wrong, anything else relevant)

---

## Issues

<!-- Start adding issues below this line -->
