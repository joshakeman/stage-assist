package domain

// CueStatus describes how a spoken cue relates to its scripted counterpart.
type CueStatus string

const (
	StatusExact   CueStatus = "exact"
	StatusChanged CueStatus = "changed"
	StatusMissing CueStatus = "missing"
	StatusExtra   CueStatus = "extra"
)

// LineNote is one row of feedback: a scripted cue, what was actually said
// (if anything), and the word-level diff between them.
type LineNote struct {
	Index      int
	Status     CueStatus
	ScriptText string
	SpokenText string
	Diff       []WordDiffSpan
}

// Align compares two ordered cue sequences and returns line notes describing
// where they match, changed, or were dropped/added. It has no notion of
// "character" or a single focal speaker — that filtering happens once in
// ParseCues before either sequence reaches Align, so Align works the same
// whether it's given one speaker's lines or, later, a whole scene's.
//
// Alignment is plain LCS over cues (equal cues match exactly, by normalized
// text); it does not detect transposition, so a pair of reordered cues is
// reported as one missing cue plus one extra cue, not a "moved" note.
func Align(script, transcript []Cue) []LineNote {
	steps := lcsDiff(script, transcript, func(a, b Cue) bool {
		return normalizeCueText(a.Text) == normalizeCueText(b.Text)
	})

	var notes []LineNote
	for i := 0; i < len(steps); {
		if steps[i].Op == OpEqual {
			notes = append(notes, exactNote(script, transcript, steps[i]))
			i++
			continue
		}
		gapStart := i
		for i < len(steps) && steps[i].Op != OpEqual {
			i++
		}
		notes = append(notes, alignGap(script, transcript, steps[gapStart:i])...)
	}

	for idx := range notes {
		notes[idx].Index = idx
	}
	return notes
}

func exactNote(script, transcript []Cue, s opStep) LineNote {
	scriptText := script[s.AIdx].Text
	spokenText := transcript[s.BIdx].Text
	return LineNote{
		Status:     StatusExact,
		ScriptText: scriptText,
		SpokenText: spokenText,
		Diff:       WordDiff(Tokenize(scriptText), Tokenize(spokenText)),
	}
}

// alignGap turns a maximal run of consecutive delete/insert steps (a "gap"
// between two equal cues) into line notes. Deletes and inserts are paired
// positionally — the first delete with the first insert, the second with the
// second, and so on — and a pair becomes a single "changed" note only if the
// two cues share at least one normalized word (a cheap similarity gate).
// Anything left unpaired, or paired but sharing no words, is reported as
// separate missing/extra notes rather than a false "changed" pairing.
func alignGap(script, transcript []Cue, gap []opStep) []LineNote {
	var deletes, inserts []int
	for _, s := range gap {
		if s.Op == OpDelete {
			deletes = append(deletes, s.AIdx)
		} else {
			inserts = append(inserts, s.BIdx)
		}
	}

	paired := min(len(deletes), len(inserts))

	var notes []LineNote
	for k := 0; k < paired; k++ {
		scriptText := script[deletes[k]].Text
		spokenText := transcript[inserts[k]].Text
		if sharesToken(scriptText, spokenText) {
			notes = append(notes, LineNote{
				Status:     StatusChanged,
				ScriptText: scriptText,
				SpokenText: spokenText,
				Diff:       WordDiff(Tokenize(scriptText), Tokenize(spokenText)),
			})
		} else {
			notes = append(notes, missingNote(scriptText), extraNote(spokenText))
		}
	}
	for _, di := range deletes[paired:] {
		notes = append(notes, missingNote(script[di].Text))
	}
	for _, ii := range inserts[paired:] {
		notes = append(notes, extraNote(transcript[ii].Text))
	}
	return notes
}

func missingNote(scriptText string) LineNote {
	return LineNote{Status: StatusMissing, ScriptText: scriptText, Diff: []WordDiffSpan{}}
}

func extraNote(spokenText string) LineNote {
	return LineNote{Status: StatusExtra, SpokenText: spokenText, Diff: []WordDiffSpan{}}
}

// sharesToken reports whether a and b have at least one normalized word in
// common.
func sharesToken(a, b string) bool {
	seen := make(map[string]bool)
	for _, t := range Tokenize(a) {
		seen[t.Norm] = true
	}
	for _, t := range Tokenize(b) {
		if seen[t.Norm] {
			return true
		}
	}
	return false
}
