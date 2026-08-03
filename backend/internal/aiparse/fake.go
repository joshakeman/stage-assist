package aiparse

import (
	"context"

	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

// FakeInterpreter is a canned ScriptInterpreter for fast, deterministic
// tests. It never makes a network call, and it never runs Verify itself --
// it returns exactly Script/Err as configured, so callers testing
// downstream logic (e.g. how an HTTP handler reacts to a particular mix of
// verified/unverified elements) can construct that exact CandidateScript
// directly rather than needing a real page-grounded response.
//
// Called records whether InterpretScript was ever invoked, so a test can
// assert the interpreter was correctly never reached at all -- e.g. when a
// scanned PDF should be rejected before any Claude call is made.
type FakeInterpreter struct {
	Script CandidateScript
	Err    error
	Called bool
}

func (f *FakeInterpreter) InterpretScript(context.Context, []pdftext.PageText) (CandidateScript, error) {
	f.Called = true
	return f.Script, f.Err
}
