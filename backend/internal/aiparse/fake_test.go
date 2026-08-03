package aiparse_test

import (
	"context"
	"errors"
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

func TestFakeInterpreterReturnsConfiguredScript(t *testing.T) {
	want := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{Kind: domain.KindDialogue, Character: "HAMLET", Text: "To be or not to be", Verified: true, Page: 1},
	}}
	fake := &aiparse.FakeInterpreter{Script: want}

	got, err := fake.InterpretScript(context.Background(), nil)
	if err != nil {
		t.Fatalf("InterpretScript: %v", err)
	}
	if len(got.Elements) != 1 || got.Elements[0] != want.Elements[0] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFakeInterpreterReturnsConfiguredError(t *testing.T) {
	wantErr := errors.New("boom")
	fake := &aiparse.FakeInterpreter{Err: wantErr}

	_, err := fake.InterpretScript(context.Background(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestFakeInterpreterRecordsWhetherItWasCalled(t *testing.T) {
	fake := &aiparse.FakeInterpreter{}
	if fake.Called {
		t.Fatal("Called = true before any invocation")
	}

	fake.InterpretScript(context.Background(), nil)
	if !fake.Called {
		t.Error("Called = false after InterpretScript ran")
	}
}
