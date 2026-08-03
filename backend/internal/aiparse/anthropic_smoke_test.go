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
// API. Originally the Stage B1 checkpoint proving the tool-use schema
// round-trips into a usable CandidateScript; now that InterpretScript also
// runs Verify (Stage B2), this doubles as confirmation that the grounding
// logic behaves sensibly against a real response, not just synthetic
// fixtures.
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

	var sawHamletDialogue, sawAnyVerified bool
	for _, el := range got.Elements {
		t.Logf("element: kind=%s character=%q text=%q evidence=%q page=%d verified=%v",
			el.Kind, el.Character, el.Text, el.SourceEvidence, el.Page, el.Verified)
		if el.Kind == domain.KindDialogue && strings.EqualFold(el.Character, "HAMLET") {
			sawHamletDialogue = true
		}
		if el.Verified {
			sawAnyVerified = true
			if el.Page != 1 {
				t.Errorf("Page = %d, want 1 (this fixture is a single-page PDF)", el.Page)
			}
		}
	}
	if !sawHamletDialogue {
		t.Error("expected at least one dialogue element attributed to HAMLET")
	}
	if !sawAnyVerified {
		t.Error("expected at least one element to be Verified against the real extracted text")
	}
}
