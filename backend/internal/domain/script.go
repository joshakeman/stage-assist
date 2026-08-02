package domain

import (
	"regexp"
	"strings"
	"unicode"
)

// ElementKind identifies what kind of content a ScriptElement holds.
type ElementKind string

const (
	// KindDialogue is a line (or run of lines) of dialogue attributed to a
	// character.
	KindDialogue ElementKind = "dialogue"
	// KindDirection is a standalone stage direction: a line that, in its
	// entirety, is a single (...) or [...] span, e.g. "(He pauses.)".
	KindDirection ElementKind = "direction"
	// KindUnclassified is a non-blank line that isn't recognized dialogue or
	// a standalone direction. It carries no claim about what it actually is
	// (a scene heading, unbracketed stage direction, or something else) --
	// there is no reliable syntactic signal to tell those apart yet.
	KindUnclassified ElementKind = "unclassified"
)

// ScriptElement is one structural unit of a parsed script.
type ScriptElement struct {
	Kind      ElementKind
	Character string // set only when Kind == KindDialogue
	Text      string
	StartLine int // 1-based line of the first physical line in this element
	EndLine   int // 1-based line of the last physical line in this element
}

// Script is the canonical, format-agnostic representation every parser
// (plain text today; Fountain, PDF, DOCX later) must produce. Nothing
// downstream of a parser (ExtractCues, Align) knows anything about source
// format -- only a ScriptElement's Kind/Character/Text.
type Script struct {
	Elements []ScriptElement
}

var (
	parenthetical = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	bracketedLine = regexp.MustCompile(`^\(.*\)$|^\[.*\]$`)
)

// ParsePlainTextScript parses the "CHARACTER: dialogue" convention, walking
// lines in order with the first matching rule winning:
//
//  1. A blank line ends any open continuation and produces no element.
//  2. A line that is, in its entirety, a single (...) or [...] span is a
//     standalone direction -- checked before continuation, so a direction
//     line occurring mid-speech is never mistaken for more dialogue.
//  3. A line with a "NAME:" prefix, where NAME contains no lowercase
//     letters, starts a new dialogue cue (see isNewCueLine for why case
//     matters once continuation exists).
//  4. Any other non-blank line continues the currently open dialogue cue,
//     if one is open.
//  5. Otherwise it becomes an unclassified element.
func ParsePlainTextScript(text string) Script {
	var script Script
	openIdx := -1 // index into script.Elements of the open dialogue element, or -1

	for i, raw := range strings.Split(text, "\n") {
		line := i + 1
		trimmed := strings.TrimSpace(raw)

		switch {
		case trimmed == "":
			openIdx = -1

		case bracketedLine.MatchString(trimmed):
			script.Elements = append(script.Elements, ScriptElement{
				Kind:      KindDirection,
				Text:      trimmed,
				StartLine: line,
				EndLine:   line,
			})
			openIdx = -1

		case isNewCueLine(raw):
			idx := strings.Index(raw, ":")
			dialogue := cleanDialogueText(raw[idx+1:])
			if dialogue == "" {
				// A bare "CHARACTER:" line with nothing to say: no element,
				// same as a blank line.
				openIdx = -1
				continue
			}
			script.Elements = append(script.Elements, ScriptElement{
				Kind:      KindDialogue,
				Character: strings.TrimSpace(raw[:idx]),
				Text:      dialogue,
				StartLine: line,
				EndLine:   line,
			})
			openIdx = len(script.Elements) - 1

		case openIdx >= 0:
			el := &script.Elements[openIdx]
			el.Text += " " + cleanDialogueText(raw)
			el.EndLine = line

		default:
			script.Elements = append(script.Elements, ScriptElement{
				Kind:      KindUnclassified,
				Text:      trimmed,
				StartLine: line,
				EndLine:   line,
			})
		}
	}
	return script
}

// isNewCueLine reports whether raw starts a new dialogue cue: it must
// contain a ':' with a non-empty label before it, and that label must
// contain no lowercase letters. The label requirement exists because once
// continuation lines are parsed alongside first lines, every line -- not
// just recognized first lines -- has to answer "is this a new cue or more
// of the last one," and dialogue commonly contains colons ("She said: come
// here"). An all-caps requirement matches this project's own (and real
// scripts') character-name convention and resolves that ambiguity: "She
// said" fails the check and is correctly treated as continuation, while
// "HAMLET" passes.
func isNewCueLine(raw string) bool {
	idx := strings.Index(raw, ":")
	if idx < 0 {
		return false
	}
	label := strings.TrimSpace(raw[:idx])
	if label == "" {
		return false
	}
	for _, r := range label {
		if unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// cleanDialogueText strips inline parenthetical/bracketed asides and
// collapses whitespace, for both a cue's first line and its continuations.
func cleanDialogueText(s string) string {
	s = parenthetical.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}
