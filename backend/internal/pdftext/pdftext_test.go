package pdftext_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatalf("opening fixture %s: %v", name, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestExtractTextColonStyle(t *testing.T) {
	pages, err := pdftext.ExtractText(openFixture(t, "colon_style.pdf"))
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if pages[0].Number != 1 {
		t.Errorf("page Number = %d, want 1", pages[0].Number)
	}
	if !strings.Contains(pages[0].Text, "HAMLET") || !strings.Contains(pages[0].Text, "Who's there?") {
		t.Errorf("extracted text missing expected content: %q", pages[0].Text)
	}
}

func TestExtractTextCenteredNameStyle(t *testing.T) {
	pages, err := pdftext.ExtractText(openFixture(t, "centered_name_style.pdf"))
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if !strings.Contains(pages[0].Text, "JULIET") || !strings.Contains(pages[0].Text, "wherefore art thou Romeo") {
		t.Errorf("extracted text missing expected content: %q", pages[0].Text)
	}
}

func TestExtractTextParentheticalStyle(t *testing.T) {
	pages, err := pdftext.ExtractText(openFixture(t, "parenthetical_monologue_style.pdf"))
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if !strings.Contains(pages[0].Text, "KING") || !strings.Contains(pages[0].Text, "crown") {
		t.Errorf("extracted text missing expected content: %q", pages[0].Text)
	}
}

func TestExtractTextScannedPDFHasNoTextLayer(t *testing.T) {
	_, err := pdftext.ExtractText(openFixture(t, "scanned_no_text_layer.pdf"))
	if !errors.Is(err, pdftext.ErrNoTextLayer) {
		t.Fatalf("err = %v, want ErrNoTextLayer", err)
	}
}

func TestExtractTextRejectsNonPDF(t *testing.T) {
	_, err := pdftext.ExtractText(openFixture(t, "not_a_pdf.pdf"))
	if !errors.Is(err, pdftext.ErrNotAPDF) {
		t.Fatalf("err = %v, want ErrNotAPDF", err)
	}
}

func TestExtractTextRejectsCorruptPDF(t *testing.T) {
	_, err := pdftext.ExtractText(openFixture(t, "corrupt.pdf"))
	if err == nil {
		t.Fatal("got nil error, want a corruption error")
	}
	if errors.Is(err, pdftext.ErrNotAPDF) {
		t.Fatalf("err = %v, want a corruption error distinct from ErrNotAPDF (this file has a valid PDF header)", err)
	}
}

// The page-count cap is tested by temporarily lowering MaxPages rather than
// committing an artificially huge fixture -- any of the small real fixtures
// has more than zero pages, so lowering the cap below that is enough to
// exercise the rejection path deterministically.
func TestExtractTextRejectsTooManyPages(t *testing.T) {
	original := pdftext.MaxPages
	pdftext.MaxPages = 0
	t.Cleanup(func() { pdftext.MaxPages = original })

	_, err := pdftext.ExtractText(openFixture(t, "colon_style.pdf"))
	if !errors.Is(err, pdftext.ErrTooManyPages) {
		t.Fatalf("err = %v, want ErrTooManyPages", err)
	}
}

// Same technique for the extracted-text-length cap.
func TestExtractTextRejectsTextTooLong(t *testing.T) {
	original := pdftext.MaxExtractedChars
	pdftext.MaxExtractedChars = 0
	t.Cleanup(func() { pdftext.MaxExtractedChars = original })

	_, err := pdftext.ExtractText(openFixture(t, "colon_style.pdf"))
	if !errors.Is(err, pdftext.ErrTextTooLong) {
		t.Fatalf("err = %v, want ErrTextTooLong", err)
	}
}
