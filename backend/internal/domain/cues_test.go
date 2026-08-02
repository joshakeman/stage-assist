package domain_test

import (
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

func TestParseCuesConsecutiveCues(t *testing.T) {
	text := "HAMLET: To be or not to be\nHAMLET: that is the question\n"

	cues := domain.ParseCues(text, "HAMLET")
	if len(cues) != 2 {
		t.Fatalf("got %d cues, want 2: %+v", len(cues), cues)
	}
	if cues[0].Text != "To be or not to be" || cues[1].Text != "that is the question" {
		t.Fatalf("unexpected cue text: %+v", cues)
	}
}

func TestParseCuesCaseInsensitiveLabel(t *testing.T) {
	text := "hamlet: To be or not to be\n"

	cues := domain.ParseCues(text, "HAMLET")
	if len(cues) != 1 {
		t.Fatalf("got %d cues, want 1: %+v", len(cues), cues)
	}
}

func TestParseCuesAbsentCharacter(t *testing.T) {
	text := "OPHELIA: My lord\n"

	cues := domain.ParseCues(text, "HAMLET")
	if len(cues) != 0 {
		t.Fatalf("got %d cues, want 0: %+v", len(cues), cues)
	}
}

func TestParseCuesStripsParentheticals(t *testing.T) {
	text := "HAMLET: (whispering) get out\n"

	cues := domain.ParseCues(text, "HAMLET")
	if len(cues) != 1 {
		t.Fatalf("got %d cues, want 1: %+v", len(cues), cues)
	}
	if cues[0].Text != "get out" {
		t.Fatalf("got text %q, want %q", cues[0].Text, "get out")
	}
}

func TestParseCuesSkipsUnlabeledLines(t *testing.T) {
	text := "ACT ONE, SCENE ONE\nHAMLET: To be or not to be\n"

	cues := domain.ParseCues(text, "HAMLET")
	if len(cues) != 1 {
		t.Fatalf("got %d cues, want 1: %+v", len(cues), cues)
	}
}
