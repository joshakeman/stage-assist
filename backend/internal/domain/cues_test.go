package domain_test

import (
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

func extractCues(t *testing.T, text, character string) []domain.Cue {
	t.Helper()
	return domain.ExtractCues(domain.ParsePlainTextScript(text), character)
}

func TestExtractCuesConsecutiveCues(t *testing.T) {
	text := "HAMLET: To be or not to be\nHAMLET: that is the question\n"

	cues := extractCues(t, text, "HAMLET")
	if len(cues) != 2 {
		t.Fatalf("got %d cues, want 2: %+v", len(cues), cues)
	}
	if cues[0].Text != "To be or not to be" || cues[1].Text != "that is the question" {
		t.Fatalf("unexpected cue text: %+v", cues)
	}
}

// Matching the requested character is case-insensitive; the script's own
// label must still be all-caps to be recognized as a cue at all (see
// isNewCueLine in script.go) -- that's a parsing rule, not an extraction
// rule, so it's covered by TestParsePlainTextScriptLowercaseLabelIsNotACue.
func TestExtractCuesCaseInsensitiveMatch(t *testing.T) {
	text := "HAMLET: To be or not to be\n"

	cues := extractCues(t, text, "hamlet")
	if len(cues) != 1 {
		t.Fatalf("got %d cues, want 1: %+v", len(cues), cues)
	}
}

func TestExtractCuesAbsentCharacter(t *testing.T) {
	text := "OPHELIA: My lord\n"

	cues := extractCues(t, text, "HAMLET")
	if len(cues) != 0 {
		t.Fatalf("got %d cues, want 0: %+v", len(cues), cues)
	}
}

func TestExtractCuesSkipsUnclassifiedElements(t *testing.T) {
	text := "ACT ONE, SCENE ONE\nHAMLET: To be or not to be\n"

	cues := extractCues(t, text, "HAMLET")
	if len(cues) != 1 {
		t.Fatalf("got %d cues, want 1: %+v", len(cues), cues)
	}
	if cues[0].Text != "To be or not to be" {
		t.Fatalf("unexpected cue text: %+v", cues)
	}
}

func TestExtractCuesDirectionsDontDisruptOrdering(t *testing.T) {
	text := "HAMLET: To be or not to be\n(He pauses.)\nHAMLET: that is the question\n"

	cues := extractCues(t, text, "HAMLET")
	if len(cues) != 2 {
		t.Fatalf("got %d cues, want 2: %+v", len(cues), cues)
	}
	if cues[0].Text != "To be or not to be" || cues[1].Text != "that is the question" {
		t.Fatalf("unexpected cue text/order: %+v", cues)
	}
}

func TestExtractCuesNoMatchingDialogueIsEmpty(t *testing.T) {
	text := "ACT ONE, SCENE ONE\n(Enter Hamlet.)\n"

	cues := extractCues(t, text, "HAMLET")
	if len(cues) != 0 {
		t.Fatalf("got %d cues, want 0: %+v", len(cues), cues)
	}
}
