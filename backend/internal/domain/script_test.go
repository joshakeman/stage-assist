package domain_test

import (
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

func TestParsePlainTextScriptSingleDialogueLine(t *testing.T) {
	script := domain.ParsePlainTextScript("HAMLET: To be or not to be\n")

	if len(script.Elements) != 1 {
		t.Fatalf("got %d elements, want 1: %+v", len(script.Elements), script.Elements)
	}
	el := script.Elements[0]
	if el.Kind != domain.KindDialogue {
		t.Errorf("Kind = %q, want %q", el.Kind, domain.KindDialogue)
	}
	if el.Character != "HAMLET" {
		t.Errorf("Character = %q, want %q", el.Character, "HAMLET")
	}
	if el.Text != "To be or not to be" {
		t.Errorf("Text = %q, want %q", el.Text, "To be or not to be")
	}
	if el.StartLine != 1 || el.EndLine != 1 {
		t.Errorf("StartLine/EndLine = %d/%d, want 1/1", el.StartLine, el.EndLine)
	}
}

func TestParsePlainTextScriptStripsParentheticals(t *testing.T) {
	script := domain.ParsePlainTextScript("HAMLET: (whispering) get out\n")

	if len(script.Elements) != 1 {
		t.Fatalf("got %d elements, want 1: %+v", len(script.Elements), script.Elements)
	}
	if got := script.Elements[0].Text; got != "get out" {
		t.Errorf("Text = %q, want %q", got, "get out")
	}
}

func TestParsePlainTextScriptUnclassifiedLine(t *testing.T) {
	script := domain.ParsePlainTextScript("ACT ONE, SCENE ONE\n")

	if len(script.Elements) != 1 {
		t.Fatalf("got %d elements, want 1: %+v", len(script.Elements), script.Elements)
	}
	el := script.Elements[0]
	if el.Kind != domain.KindUnclassified {
		t.Errorf("Kind = %q, want %q", el.Kind, domain.KindUnclassified)
	}
	if el.Text != "ACT ONE, SCENE ONE" {
		t.Errorf("Text = %q, want %q", el.Text, "ACT ONE, SCENE ONE")
	}
}

func TestParsePlainTextScriptBlankLineProducesNoElement(t *testing.T) {
	script := domain.ParsePlainTextScript("HAMLET: line one\n\nHAMLET: that is the question\n")

	if len(script.Elements) != 2 {
		t.Fatalf("got %d elements, want 2 (blank line must not produce one): %+v", len(script.Elements), script.Elements)
	}
}

func TestParsePlainTextScriptEmptyInput(t *testing.T) {
	script := domain.ParsePlainTextScript("")

	if len(script.Elements) != 0 {
		t.Fatalf("got %d elements, want 0: %+v", len(script.Elements), script.Elements)
	}
}

func TestParsePlainTextScriptJoinsContinuationLine(t *testing.T) {
	script := domain.ParsePlainTextScript("HAMLET: To be or not to be,\nthat is the question\n")

	if len(script.Elements) != 1 {
		t.Fatalf("got %d elements, want 1 (continuation must join): %+v", len(script.Elements), script.Elements)
	}
	el := script.Elements[0]
	if el.Text != "To be or not to be, that is the question" {
		t.Errorf("Text = %q, want %q", el.Text, "To be or not to be, that is the question")
	}
	if el.StartLine != 1 || el.EndLine != 2 {
		t.Errorf("StartLine/EndLine = %d/%d, want 1/2", el.StartLine, el.EndLine)
	}
}

func TestParsePlainTextScriptBlankLineEndsContinuation(t *testing.T) {
	script := domain.ParsePlainTextScript("HAMLET: To be or not to be,\n\nthat is the question\n")

	if len(script.Elements) != 2 {
		t.Fatalf("got %d elements, want 2 (blank line must end continuation): %+v", len(script.Elements), script.Elements)
	}
	if script.Elements[0].Kind != domain.KindDialogue || script.Elements[0].Text != "To be or not to be," {
		t.Errorf("element 0 = %+v", script.Elements[0])
	}
	if script.Elements[1].Kind != domain.KindUnclassified || script.Elements[1].Text != "that is the question" {
		t.Errorf("element 1 = %+v", script.Elements[1])
	}
}

func TestParsePlainTextScriptDirectionLineEndsContinuation(t *testing.T) {
	text := "HAMLET: To be or not to be,\n(He pauses.)\nHAMLET: that is the question\n"
	script := domain.ParsePlainTextScript(text)

	if len(script.Elements) != 3 {
		t.Fatalf("got %d elements, want 3 (dialogue, direction, dialogue): %+v", len(script.Elements), script.Elements)
	}
	if script.Elements[0].Kind != domain.KindDialogue || script.Elements[0].Text != "To be or not to be," {
		t.Errorf("element 0 = %+v", script.Elements[0])
	}
	if script.Elements[1].Kind != domain.KindDirection || script.Elements[1].Text != "(He pauses.)" {
		t.Errorf("element 1 = %+v", script.Elements[1])
	}
	if script.Elements[2].Kind != domain.KindDialogue || script.Elements[2].Text != "that is the question" {
		t.Errorf("element 2 = %+v", script.Elements[2])
	}
}

func TestParsePlainTextScriptBracketedDirectionLine(t *testing.T) {
	script := domain.ParsePlainTextScript("[Exit Hamlet]\n")

	if len(script.Elements) != 1 {
		t.Fatalf("got %d elements, want 1: %+v", len(script.Elements), script.Elements)
	}
	if script.Elements[0].Kind != domain.KindDirection || script.Elements[0].Text != "[Exit Hamlet]" {
		t.Errorf("element = %+v", script.Elements[0])
	}
}

func TestParsePlainTextScriptRepeatedLabelDoesNotMerge(t *testing.T) {
	script := domain.ParsePlainTextScript("HAMLET: line one\nHAMLET: line two\n")

	if len(script.Elements) != 2 {
		t.Fatalf("got %d elements, want 2 (repeated NAME: must never merge): %+v", len(script.Elements), script.Elements)
	}
	if script.Elements[0].Text != "line one" || script.Elements[1].Text != "line two" {
		t.Fatalf("unexpected element text: %+v", script.Elements)
	}
}

func TestParsePlainTextScriptLowercaseLabelIsNotACue(t *testing.T) {
	script := domain.ParsePlainTextScript("hamlet: To be or not to be\n")

	if len(script.Elements) != 1 {
		t.Fatalf("got %d elements, want 1: %+v", len(script.Elements), script.Elements)
	}
	el := script.Elements[0]
	if el.Kind != domain.KindUnclassified {
		t.Errorf("Kind = %q, want %q (lowercase label must not start a cue)", el.Kind, domain.KindUnclassified)
	}
	if el.Text != "hamlet: To be or not to be" {
		t.Errorf("Text = %q, want the whole line preserved verbatim", el.Text)
	}
}

func TestParsePlainTextScriptContinuationIsCharacterAgnostic(t *testing.T) {
	text := "HAMLET: To be or not to be,\nthat is the question\nOPHELIA: My lord,\nI have remembrances of yours\n"
	script := domain.ParsePlainTextScript(text)

	if len(script.Elements) != 2 {
		t.Fatalf("got %d elements, want 2: %+v", len(script.Elements), script.Elements)
	}
	if script.Elements[0].Character != "HAMLET" || script.Elements[0].Text != "To be or not to be, that is the question" {
		t.Errorf("element 0 = %+v", script.Elements[0])
	}
	if script.Elements[1].Character != "OPHELIA" || script.Elements[1].Text != "My lord, I have remembrances of yours" {
		t.Errorf("element 1 = %+v", script.Elements[1])
	}
}
