package aiparse

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// usageLogPath is where each real Anthropic call's token usage and
// estimated cost is appended, one JSON object per line. It's a var, not a
// const, so a test could point it elsewhere without touching a real file,
// though today nothing does -- recordUsage's file I/O isn't covered by
// fast tests, since it's a side effect deliberately kept out of
// InterpretScript's return value (see recordUsage).
var usageLogPath = "usage.jsonl"

// sonnet5PricePerMillionTokens returns Claude Sonnet 5's published
// USD-per-million-token input/output prices in effect at when. Introductory
// pricing runs through August 31, 2026; this switches to standard pricing
// automatically after that so the estimate doesn't go stale the moment the
// published rate changes. This is an estimate against list price, not a
// bill -- it doesn't know about account-specific discounts, and the
// Anthropic Console's own usage dashboard remains the authoritative source
// for actual spend.
func sonnet5PricePerMillionTokens(when time.Time) (inputUSD, outputUSD float64) {
	introductoryPricingEnds := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	if when.Before(introductoryPricingEnds) {
		return 2, 10
	}
	return 3, 15
}

// usageRecord is one real Anthropic call's cost accounting, appended to
// usageLogPath as a single JSON line.
type usageRecord struct {
	Time             time.Time `json:"time"`
	Model            string    `json:"model"`
	InputTokens      int64     `json:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
}

// recordUsage appends usage's cost accounting to usageLogPath for every
// real call, regardless of what happens to the response afterward (a
// truncated or malformed response was still billed for the tokens it
// used). It never returns an error: a logging failure must not break the
// actual import, so problems are only reported to stderr, best-effort.
//
// Only calibrated for Claude Sonnet 5, the one model this package uses
// (see NewAnthropicInterpreter) -- if a second model is ever introduced,
// this needs a per-model price lookup instead of assuming Sonnet 5's rate.
func recordUsage(model anthropic.Model, usage anthropic.Usage) {
	now := time.Now().UTC()
	inputRate, outputRate := sonnet5PricePerMillionTokens(now)

	record := usageRecord{
		Time:         now,
		Model:        string(model),
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		EstimatedCostUSD: float64(usage.InputTokens)/1_000_000*inputRate +
			float64(usage.OutputTokens)/1_000_000*outputRate,
	}

	line, err := json.Marshal(record)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aiparse: recording usage: %v\n", err)
		return
	}
	line = append(line, '\n')

	f, err := os.OpenFile(usageLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aiparse: recording usage: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		fmt.Fprintf(os.Stderr, "aiparse: recording usage: %v\n", err)
	}
}
