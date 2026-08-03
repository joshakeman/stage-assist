package aiparse_test

import (
	"errors"
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/domain"
	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

func pages(texts ...string) []pdftext.PageText {
	p := make([]pdftext.PageText, len(texts))
	for i, t := range texts {
		p[i] = pdftext.PageText{Number: i + 1, Text: t}
	}
	return p
}

func TestVerifyMarksVerbatimEvidenceAsVerified(t *testing.T) {
	script := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{Kind: domain.KindDialogue, Character: "HAMLET", Text: "Who's there?", SourceEvidence: "HAMLET: Who's there?"},
	}}

	got, err := aiparse.Verify(script, pages("HAMLET: Who's there?\nFRANCISCO: Nay, answer me."))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !got.Elements[0].Verified {
		t.Error("Verified = false, want true for verbatim evidence")
	}
	if got.Elements[0].Page != 1 {
		t.Errorf("Page = %d, want 1", got.Elements[0].Page)
	}
}

func TestVerifyAssignsPageFromWhereEvidenceIsFound(t *testing.T) {
	script := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{Kind: domain.KindDialogue, Character: "HAMLET", Text: "Long live the king!", SourceEvidence: "HAMLET: Long live the king!"},
	}}

	got, err := aiparse.Verify(script, pages("some unrelated first page", "HAMLET: Long live the king!"))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !got.Elements[0].Verified {
		t.Fatal("Verified = false, want true")
	}
	if got.Elements[0].Page != 2 {
		t.Errorf("Page = %d, want 2 (the page evidence actually appears on)", got.Elements[0].Page)
	}
}

// This is the regression test for the vulnerability found in review: a
// plain "does the evidence exist somewhere" check can be beaten by pairing
// a real, trivial anchor with a fully invented Text. Verify must catch
// this via the text-evidence-consistency check, not just groundedness.
func TestVerifyRejectsInventedTextPairedWithARealAnchor(t *testing.T) {
	script := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{
			Kind:           domain.KindDialogue,
			Character:      "ROMEO",
			Text:           "I will fight to the death for you",
			SourceEvidence: "ROMEO",
		},
		// A second, genuinely verified element so this test exercises the
		// "flagged but not dropped" path rather than ErrNothingVerified.
		{Kind: domain.KindDialogue, Character: "JULIET", Text: "Good night.", SourceEvidence: "JULIET: Good night."},
	}}

	got, err := aiparse.Verify(script, pages("ROMEO enters.\nJULIET: Good night."))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(got.Elements) != 2 {
		t.Fatalf("got %d elements, want 2 (nothing should be dropped)", len(got.Elements))
	}
	if got.Elements[0].Verified {
		t.Error("Verified = true for an invented Text paired with an unrelated real anchor, want false")
	}
	if !got.Elements[1].Verified {
		t.Error("Verified = false for a genuinely grounded element, want true")
	}
}

func TestVerifyTreatesLineWrapHyphenationAsEquivalent(t *testing.T) {
	script := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{Kind: domain.KindDialogue, Character: "KING", Text: "He must understand this fully.", SourceEvidence: "He must understand this fully."},
	}}

	got, err := aiparse.Verify(script, pages("KING: He must under-\nstand this fully."))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !got.Elements[0].Verified {
		t.Error("Verified = false, want true: a mid-word line-wrap hyphen should not defeat grounding")
	}
}

func TestVerifyTreatesSmartQuotesAndDashesAsEquivalent(t *testing.T) {
	script := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{Kind: domain.KindDialogue, Character: "HAMLET", Text: "'Tis a fine day - truly.", SourceEvidence: "'Tis a fine day - truly."},
	}}

	got, err := aiparse.Verify(script, pages("HAMLET: ’Tis a fine day — truly."))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !got.Elements[0].Verified {
		t.Error("Verified = false, want true: smart quotes/dashes should normalize the same as their ASCII equivalents")
	}
}

// Regression test for a real finding during manual testing: a Hermia
// speech in a real script excerpt legitimately spanned a physical page
// break, and was wrongly reported as unverified because grounding checked
// each page's text in isolation instead of the whole document. The
// trailing "2" mirrors the real fixture exactly: a printed page-footer
// number that PDF extraction picks up as the page's last token, sitting
// right at the join point.
func TestVerifyFindsEvidenceSpanningAPageBreak(t *testing.T) {
	script := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{
			Kind:           domain.KindDialogue,
			Character:      "HERMIA",
			Text:           "I do entreat your grace to pardon me, but I beseech your grace that I may know.",
			SourceEvidence: "I do entreat your grace to pardon me,\nBut I beseech your grace that I may know.",
		},
	}}

	got, err := aiparse.Verify(script, pages(
		"HERMIA: I do entreat your grace to pardon me,\n2",
		"But I beseech your grace that I may know.",
	))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !got.Elements[0].Verified {
		t.Error("Verified = false, want true: evidence spanning a page break (past a page-footer number) should still be found")
	}
	if got.Elements[0].Page != 1 {
		t.Errorf("Page = %d, want 1 (the page the evidence starts on)", got.Elements[0].Page)
	}
}

func TestVerifyReturnsErrNothingVerifiedWhenEverythingFails(t *testing.T) {
	script := aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{Kind: domain.KindDialogue, Character: "HAMLET", Text: "made up line", SourceEvidence: "not in the source at all"},
	}}

	_, err := aiparse.Verify(script, pages("Completely unrelated content."))
	if !errors.Is(err, aiparse.ErrNothingVerified) {
		t.Fatalf("err = %v, want ErrNothingVerified", err)
	}
}
