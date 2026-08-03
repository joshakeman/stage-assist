package aiparse

// White-box (package aiparse), not aiparse_test, because usageLogPath,
// usageRecord, and sonnet5PricePerMillionTokens are internal implementation
// details with no exported surface -- the same justification as
// diff_internal_test.go in internal/domain.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

func TestSonnet5PricePerMillionTokensAroundTheCutover(t *testing.T) {
	before := time.Date(2026, time.August, 31, 23, 59, 59, 0, time.UTC)
	if in, out := sonnet5PricePerMillionTokens(before); in != 2 || out != 10 {
		t.Errorf("just before cutover: got (%v, %v), want (2, 10)", in, out)
	}

	atCutover := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	if in, out := sonnet5PricePerMillionTokens(atCutover); in != 3 || out != 15 {
		t.Errorf("at cutover: got (%v, %v), want (3, 15)", in, out)
	}
}

func TestRecordUsageAppendsOneJSONLinePerCall(t *testing.T) {
	original := usageLogPath
	usageLogPath = filepath.Join(t.TempDir(), "usage.jsonl")
	t.Cleanup(func() { usageLogPath = original })

	recordUsage(anthropic.ModelClaudeSonnet5, anthropic.Usage{InputTokens: 1000, OutputTokens: 500})
	recordUsage(anthropic.ModelClaudeSonnet5, anthropic.Usage{InputTokens: 200, OutputTokens: 60})

	data, err := os.ReadFile(usageLogPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (one append per call, not overwritten)", len(lines))
	}

	var first usageRecord
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("invalid JSON line: %v; got %q", err, lines[0])
	}
	if first.InputTokens != 1000 || first.OutputTokens != 500 {
		t.Errorf("record = %+v, want InputTokens=1000 OutputTokens=500", first)
	}
	if first.Model != string(anthropic.ModelClaudeSonnet5) {
		t.Errorf("Model = %q, want %q", first.Model, anthropic.ModelClaudeSonnet5)
	}
	if first.EstimatedCostUSD <= 0 {
		t.Errorf("EstimatedCostUSD = %v, want > 0 for non-zero token usage", first.EstimatedCostUSD)
	}
}
