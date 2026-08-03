package aiparse_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/domain"
	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

// evalCase is one real-API check against a fixture whose expected output has
// already been observed against the live API (see manual-testing/*.transcript.txt
// and this project's PDF ingestion plan). Assertions are structural lower
// bounds, not exact matches -- Claude's output isn't byte-for-byte
// deterministic across calls.
type evalCase struct {
	name     string
	pdfPath  string
	minLines map[string]int // character -> minimum expected dialogue elements
}

var evalCases = []evalCase{
	{
		name:    "colon_style_baseline",
		pdfPath: "../pdftext/testdata/colon_style.pdf",
		minLines: map[string]int{
			"HAMLET":    2,
			"FRANCISCO": 4,
			"BERNARDO":  2,
		},
	},
	{
		name:    "centered_name_style",
		pdfPath: "../pdftext/testdata/centered_name_style.pdf",
		minLines: map[string]int{
			"JULIET": 2,
			"ROMEO":  2,
		},
	},
	{
		name:    "parenthetical_monologue_style",
		pdfPath: "../pdftext/testdata/parenthetical_monologue_style.pdf",
		minLines: map[string]int{
			"KING": 3,
		},
	},
}

// TestEvalSuite is the opt-in real-API evaluation suite for aiparse. Unlike
// TestAnthropicInterpreterRealSmoke (which only checks that one fixture
// round-trips at all), this checks structural expectations across the
// layouts the deterministic plain-text parser can't handle on its own --
// the actual case for AI-assisted parsing existing at all.
//
// Skipped unless ANTHROPIC_API_KEY is set. Never part of the default suite:
// it costs real money, has real latency, and Claude's output has some
// inherent non-determinism.
//
//	go test ./internal/aiparse/... -run TestEvalSuite -v
func TestEvalSuite(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("set ANTHROPIC_API_KEY to run the real-API eval suite")
	}

	interp := aiparse.NewAnthropicInterpreter(apiKey)

	for _, tc := range evalCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.Open(tc.pdfPath)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			pages, err := pdftext.ExtractText(f)
			if err != nil {
				t.Fatalf("ExtractText: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			got, err := interp.InterpretScript(ctx, pages)
			if err != nil {
				t.Fatalf("InterpretScript: %v", err)
			}

			dialogueCounts := map[string]int{}
			verifiedCount := 0
			for _, el := range got.Elements {
				if el.Kind == domain.KindDialogue {
					dialogueCounts[el.Character]++
				}
				if el.Verified {
					verifiedCount++
				}
			}
			t.Logf("elements=%d verified=%d dialogue-counts=%v", len(got.Elements), verifiedCount, dialogueCounts)

			if verifiedCount == 0 {
				t.Error("expected at least one verified element, got zero -- this would also fail InterpretScript's own aggregate check")
			}
			for character, min := range tc.minLines {
				if dialogueCounts[character] < min {
					t.Errorf("character %q: got %d dialogue elements, want at least %d", character, dialogueCounts[character], min)
				}
			}
		})
	}
}
