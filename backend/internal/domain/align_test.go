package domain_test

import (
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

func cuesFrom(texts ...string) []domain.Cue {
	cues := make([]domain.Cue, len(texts))
	for i, text := range texts {
		cues[i] = domain.Cue{Text: text}
	}
	return cues
}

func wantStatuses(t *testing.T, notes []domain.LineNote, want ...domain.CueStatus) {
	t.Helper()
	if len(notes) != len(want) {
		t.Fatalf("got %d notes, want %d: %+v", len(notes), len(want), notes)
	}
	for i, w := range want {
		if notes[i].Status != w {
			t.Errorf("note[%d].Status = %q, want %q (%+v)", i, notes[i].Status, w, notes[i])
		}
		if notes[i].Index != i {
			t.Errorf("note[%d].Index = %d, want %d", i, notes[i].Index, i)
		}
	}
}

func TestAlignIdenticalIsAllExact(t *testing.T) {
	script := cuesFrom("Line one", "Line two", "Line three")
	transcript := cuesFrom("Line one", "Line two", "Line three")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact, domain.StatusExact, domain.StatusExact)
}

func TestAlignSingleDroppedCueMidSequence(t *testing.T) {
	script := cuesFrom("Line one", "Line two", "Line three")
	transcript := cuesFrom("Line one", "Line three")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact, domain.StatusMissing, domain.StatusExact)
	if notes[1].ScriptText != "Line two" {
		t.Errorf("missing note ScriptText = %q, want %q", notes[1].ScriptText, "Line two")
	}
	// The cue after the drop must still be exact, not a cascading false "changed".
	if notes[2].Status != domain.StatusExact {
		t.Errorf("cue after drop = %q, want exact (no cascade)", notes[2].Status)
	}
}

func TestAlignSingleAddedCueMidSequence(t *testing.T) {
	script := cuesFrom("Line one", "Line three")
	transcript := cuesFrom("Line one", "Line two", "Line three")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact, domain.StatusExtra, domain.StatusExact)
	if notes[1].SpokenText != "Line two" {
		t.Errorf("extra note SpokenText = %q, want %q", notes[1].SpokenText, "Line two")
	}
}

func TestAlignTwoConsecutiveDroppedCues(t *testing.T) {
	script := cuesFrom("Line one", "Line two", "Line three", "Line four")
	transcript := cuesFrom("Line one", "Line four")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact, domain.StatusMissing, domain.StatusMissing, domain.StatusExact)
	if notes[1].ScriptText != "Line two" || notes[2].ScriptText != "Line three" {
		t.Errorf("unexpected missing texts: %+v", notes[1:3])
	}
}

func TestAlignTwoConsecutiveAddedCues(t *testing.T) {
	script := cuesFrom("Line one", "Line four")
	transcript := cuesFrom("Line one", "Line two", "Line three", "Line four")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact, domain.StatusExtra, domain.StatusExtra, domain.StatusExact)
	if notes[1].SpokenText != "Line two" || notes[2].SpokenText != "Line three" {
		t.Errorf("unexpected extra texts: %+v", notes[1:3])
	}
}

func TestAlignParaphraseIsChanged(t *testing.T) {
	script := cuesFrom("Whether tis nobler")
	transcript := cuesFrom("Whether it's nobler")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusChanged)

	want := []domain.WordDiffSpan{
		{Op: domain.OpEqual, Text: "Whether"},
		{Op: domain.OpDelete, Text: "tis"},
		{Op: domain.OpInsert, Text: "it's"},
		{Op: domain.OpEqual, Text: "nobler"},
	}
	if len(notes[0].Diff) != len(want) {
		t.Fatalf("diff = %+v, want %+v", notes[0].Diff, want)
	}
	for i := range want {
		if notes[0].Diff[i] != want[i] {
			t.Errorf("diff[%d] = %+v, want %+v", i, notes[0].Diff[i], want[i])
		}
	}
}

func TestAlignUnrelatedDropAndAddAreNotPairedAsChanged(t *testing.T) {
	script := cuesFrom("Enter Hamlet", "The slings and arrows", "Exit Hamlet")
	transcript := cuesFrom("Enter Hamlet", "Um what was that line", "Exit Hamlet")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact, domain.StatusMissing, domain.StatusExtra, domain.StatusExact)
	if notes[1].ScriptText != "The slings and arrows" {
		t.Errorf("missing note ScriptText = %q", notes[1].ScriptText)
	}
	if notes[2].SpokenText != "Um what was that line" {
		t.Errorf("extra note SpokenText = %q", notes[2].SpokenText)
	}
}

// Regression test: two unrelated cues that happen to share only a stop
// word ("the") must not be paired as "changed" -- the pairing gate must
// require a shared *content* word, not just any shared word.
func TestAlignUnrelatedCuesSharingOnlyStopWordsAreMissingPlusExtra(t *testing.T) {
	script := cuesFrom("The crown belongs to me.")
	transcript := cuesFrom("The door is open.")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusMissing, domain.StatusExtra)
	if notes[0].ScriptText != "The crown belongs to me." {
		t.Errorf("missing note ScriptText = %q", notes[0].ScriptText)
	}
	if notes[1].SpokenText != "The door is open." {
		t.Errorf("extra note SpokenText = %q", notes[1].SpokenText)
	}
}

// A realistic paraphrase that swaps out a pronoun for a noun phrase must
// still be recognized as "changed" -- stop-word filtering must not be so
// aggressive that it also discards the real shared content words ("bring",
// "forward") that make this pairing correct.
func TestAlignParaphraseSharingContentWordsIsChanged(t *testing.T) {
	script := cuesFrom("Bring the prisoner forward.")
	transcript := cuesFrom("Bring him forward.")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusChanged)
}

func TestAlignReorderedCuesAreMissingPlusExtra(t *testing.T) {
	script := cuesFrom("Cue Alpha", "Cue Beta")
	transcript := cuesFrom("Cue Beta", "Cue Alpha")

	notes := domain.Align(script, transcript)
	// Documented limitation: transposition is not detected as a "moved" cue.
	wantStatuses(t, notes, domain.StatusMissing, domain.StatusExact, domain.StatusExtra)
	if notes[0].ScriptText != "Cue Alpha" {
		t.Errorf("missing note ScriptText = %q, want %q", notes[0].ScriptText, "Cue Alpha")
	}
	if notes[2].SpokenText != "Cue Alpha" {
		t.Errorf("extra note SpokenText = %q, want %q", notes[2].SpokenText, "Cue Alpha")
	}
}

func TestAlignWhitespaceAndPunctuationOnlyIsExact(t *testing.T) {
	script := cuesFrom("To be, or not to be.")
	transcript := cuesFrom("to be or not to be")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact)
	if notes[0].ScriptText != "To be, or not to be." {
		t.Errorf("ScriptText = %q, want original surface text preserved", notes[0].ScriptText)
	}
	if notes[0].SpokenText != "to be or not to be" {
		t.Errorf("SpokenText = %q, want original surface text preserved", notes[0].SpokenText)
	}
}

// Regression test: a cue whose dialogue itself contains a colon, spoken
// with a comma instead, must still report ScriptText and SpokenText as
// their own distinct, original surface strings -- neither one should be
// silently replaced by the other just because the cue is normalized-equal.
func TestAlignExactNoteKeepsDistinctSurfaceTextWithInternalColon(t *testing.T) {
	script := cuesFrom("Wait: did you hear that?")
	transcript := cuesFrom("Wait, did you hear that?")

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusExact)
	if notes[0].ScriptText != "Wait: did you hear that?" {
		t.Errorf("ScriptText = %q, want %q", notes[0].ScriptText, "Wait: did you hear that?")
	}
	if notes[0].SpokenText != "Wait, did you hear that?" {
		t.Errorf("SpokenText = %q, want %q", notes[0].SpokenText, "Wait, did you hear that?")
	}
}

func TestAlignEmptyTranscriptIsAllMissingNoError(t *testing.T) {
	script := cuesFrom("Line one", "Line two", "Line three")
	var transcript []domain.Cue

	notes := domain.Align(script, transcript)
	wantStatuses(t, notes, domain.StatusMissing, domain.StatusMissing, domain.StatusMissing)
	for i, want := range []string{"Line one", "Line two", "Line three"} {
		if notes[i].ScriptText != want {
			t.Errorf("note[%d].ScriptText = %q, want %q", i, notes[i].ScriptText, want)
		}
	}
}
