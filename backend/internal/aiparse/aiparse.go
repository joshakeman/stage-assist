// Package aiparse turns a script excerpt's raw extracted text into a
// candidate canonical Script, using Claude for the ambiguous interpretation
// work that a deterministic parser can't do. Nothing here is trusted as
// final: CandidateElement is deliberately not domain.ScriptElement, and
// this package never touches internal/domain's comparison types.
package aiparse

import (
	"context"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

// CandidateElement is one element of Claude's candidate interpretation of a
// script excerpt. Kind reuses domain.ElementKind rather than duplicating
// it, since it's the same three-value taxonomy either way.
//
// Page and Verified are Go-computed trust signals, never taken from
// Claude's own response -- they're added by validation logic, not by this
// stage, so they're zero-valued here until that logic exists.
type CandidateElement struct {
	Kind           domain.ElementKind
	Character      string // set only when Kind == domain.KindDialogue
	Text           string // cleaned candidate content, shown to the user
	SourceEvidence string // the span of raw text this was derived from
	Page           int    // which page SourceEvidence was found on
	Verified       bool   // whether Go could confirm SourceEvidence against the source
}

// CandidateScript is Claude's full candidate interpretation of one script
// excerpt -- not a domain.Script, and not yet trusted.
type CandidateScript struct {
	Elements []CandidateElement
}

// ScriptInterpreter turns raw per-page PDF text into a CandidateScript.
// This is the one interface in this project's ingestion path: unlike PDF
// extraction (a single, local, deterministic call site), a real
// implementation here means a real network call with real cost and
// latency, so a fast, deterministic fake is worth a seam to swap in for
// tests.
type ScriptInterpreter interface {
	InterpretScript(ctx context.Context, pages []pdftext.PageText) (CandidateScript, error)
}
