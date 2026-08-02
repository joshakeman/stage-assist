package aiparse_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/domain"
	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

// TestAnthropicInterpreterRealSmoke makes one real call to the Anthropic
// API. This is the Stage B1 checkpoint: proving the tool-use schema
// actually round-trips into a usable CandidateScript BEFORE building the
// fake implementation, grounding validation, and fast test suite around a
// merely-assumed shape.
//
// Skipped unless ANTHROPIC_API_KEY is set -- it costs real money and isn't
// fully deterministic, so it must never be part of the default fast suite.
// Run it explicitly with:
//
//	go test ./internal/aiparse/... -run TestAnthropicInterpreterRealSmoke -v
func TestAnthropicInterpreterRealSmoke(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("set ANTHROPIC_API_KEY to run this real-API smoke test")
	}

	f, err := os.Open("../pdftext/testdata/colon_style.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	pages, err := pdftext.ExtractText(f)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}

	interp := aiparse.NewAnthropicInterpreter(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	got, err := interp.InterpretScript(ctx, pages)
	if err != nil {
		t.Fatalf("InterpretScript: %v", err)
	}

	if len(got.Elements) == 0 {
		t.Fatal("got zero elements, want at least one")
	}

	var sawHamletDialogue bool
	for _, el := range got.Elements {
		t.Logf("element: kind=%s character=%q text=%q evidence=%q",
			el.Kind, el.Character, el.Text, el.SourceEvidence)
		if el.Kind == domain.KindDialogue && strings.EqualFold(el.Character, "HAMLET") {
			sawHamletDialogue = true
		}
	}
	if !sawHamletDialogue {
		t.Error("expected at least one dialogue element attributed to HAMLET")
	}
}
