// Package pdftext extracts plain text from text-based PDFs. It has no
// notion of scripts, characters, or AI -- it only turns PDF bytes into
// per-page plain text, or a clear error.
package pdftext

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// MaxPages and MaxExtractedChars bound this slice to short excerpts, not
// full-length scripts -- long-document chunking is explicitly out of scope
// for now (see the ingestion plan). They're vars, not consts, so tests can
// temporarily lower them to exercise the cap without needing enormous
// fixture files.
var (
	MaxPages          = 15
	MaxExtractedChars = 20_000
)

// minWordsForTextLayer is the floor below which extracted text is treated
// as "no meaningful text layer" -- the deliberate rejection point for
// scanned/image-only PDFs, since OCR is not supported.
var minWordsForTextLayer = 10

var (
	// ErrNotAPDF means the input doesn't start with a PDF file signature.
	ErrNotAPDF = errors.New("pdftext: input is not a PDF file")
	// ErrNoTextLayer means the PDF has no meaningful embedded text, most
	// likely because it's a scanned/image-only document.
	ErrNoTextLayer = errors.New("pdftext: no embedded text layer detected (scanned PDFs are not supported)")
	// ErrTooManyPages means the document exceeds MaxPages.
	ErrTooManyPages = errors.New("pdftext: document has too many pages for this slice")
	// ErrTextTooLong means the extracted text exceeds MaxExtractedChars.
	ErrTextTooLong = errors.New("pdftext: extracted text is too long for this slice")
)

// PageText is one page's extracted plain text, 1-based.
type PageText struct {
	Number int
	Text   string
}

// ExtractText reads a PDF from r and returns its text, one entry per page.
// It rejects input that isn't a valid PDF, PDFs with no meaningful text
// layer, and PDFs exceeding this slice's short-excerpt limits.
func ExtractText(r io.Reader) (pages []PageText, err error) {
	defer func() {
		// Parsing untrusted, potentially malformed binary PDF data with a
		// third-party library is a concrete, foreseeable panic source --
		// unlike the rest of this codebase's handlers, which only touch
		// already-validated strings and don't need this.
		if rec := recover(); rec != nil {
			pages = nil
			err = fmt.Errorf("pdftext: recovered from panic while parsing PDF: %v", rec)
		}
	}()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("pdftext: reading input: %w", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, ErrNotAPDF
	}

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("pdftext: invalid or corrupt PDF: %w", err)
	}

	numPages := reader.NumPage()
	if numPages > MaxPages {
		return nil, ErrTooManyPages
	}

	pages = make([]PageText, 0, numPages)
	totalChars := 0
	totalWords := 0
	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("pdftext: extracting page %d: %w", i, err)
		}
		pages = append(pages, PageText{Number: i, Text: text})
		totalChars += len(text)
		totalWords += len(strings.Fields(text))
	}

	if totalChars > MaxExtractedChars {
		return nil, ErrTextTooLong
	}
	if totalWords < minWordsForTextLayer {
		return nil, ErrNoTextLayer
	}

	return pages, nil
}
