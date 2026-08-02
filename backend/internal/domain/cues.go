package domain

import (
	"regexp"
	"strings"
)

// Cue is a single line of dialogue attributed to a character, in source order.
type Cue struct {
	Character string
	Text      string
	Line      int
}

var parenthetical = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)

// ParseCues extracts, in order, every cue spoken by character from text.
// Lines are expected in "CHARACTER: dialogue" form; lines without a
// recognizable "NAME:" prefix (scene headers, stage directions) are skipped.
// Character matching is case-insensitive but otherwise exact, so
// "HAMLET (O.S.):" is treated as a different label from "HAMLET:".
func ParseCues(text string, character string) []Cue {
	character = strings.TrimSpace(character)

	var cues []Cue
	for i, raw := range strings.Split(text, "\n") {
		idx := strings.Index(raw, ":")
		if idx < 0 {
			continue
		}
		label := strings.TrimSpace(raw[:idx])
		if !strings.EqualFold(label, character) {
			continue
		}

		spoken := parenthetical.ReplaceAllString(raw[idx+1:], "")
		spoken = strings.Join(strings.Fields(spoken), " ")
		if spoken == "" {
			continue
		}

		cues = append(cues, Cue{Character: label, Text: spoken, Line: i + 1})
	}
	return cues
}
