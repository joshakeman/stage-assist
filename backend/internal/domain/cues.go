package domain

import "strings"

// Cue is a single line of dialogue attributed to a character, in source order.
type Cue struct {
	Character string
	Text      string
	StartLine int
	EndLine   int
}

// ExtractCues walks a parsed Script and returns, in order, every dialogue
// element spoken by character. Character matching is case-insensitive but
// otherwise exact, so "HAMLET (O.S.)" is treated as a different label from
// "HAMLET". ExtractCues has no notion of source format -- it works
// identically no matter which parser produced script.
func ExtractCues(script Script, character string) []Cue {
	character = strings.TrimSpace(character)

	var cues []Cue
	for _, el := range script.Elements {
		if el.Kind != KindDialogue {
			continue
		}
		if !strings.EqualFold(el.Character, character) {
			continue
		}
		cues = append(cues, Cue{
			Character: el.Character,
			Text:      el.Text,
			StartLine: el.StartLine,
			EndLine:   el.EndLine,
		})
	}
	return cues
}
