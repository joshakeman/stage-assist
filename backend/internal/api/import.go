package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/pdftext"
)

// MaxUploadBytes bounds the raw multipart upload -- a separate, earlier
// check than pdftext's own page-count/extracted-text caps, since those are
// PDF-content-derived and this is a pure HTTP-transport concern. It's a
// var, not a const, so tests can lower it and exercise the rejection path
// without needing to construct a multi-megabyte fixture.
var MaxUploadBytes int64 = 5 * 1024 * 1024

// candidateElementDTO is the full, review-time shape returned by the
// import endpoint -- unlike confirmedElementDTO (handlers.go), it still
// carries the trust signals a human reviewer needs (Page, Verified).
type candidateElementDTO struct {
	Kind      string `json:"kind"`
	Character string `json:"character"`
	Text      string `json:"text"`
	Page      int    `json:"page"`
	Verified  bool   `json:"verified"`
}

type importResponse struct {
	Elements []candidateElementDTO `json:"elements"`
}

// handleScriptImport reads an uploaded PDF, extracts its text, sends it to
// the AI interpreter, and returns every resulting candidate element --
// verified or not; nothing is dropped here, only flagged. See
// aiparse.Verify for what "verified" means and doesn't mean.
func (s *server) handleScriptImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)

	if err := r.ParseMultipartForm(MaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("upload too large or malformed (max %d bytes)", MaxUploadBytes))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, `missing "file" field in upload`)
		return
	}
	defer file.Close()

	pages, err := pdftext.ExtractText(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, pdfTextErrorMessage(err))
		return
	}

	candidate, err := s.interpreter.InterpretScript(r.Context(), pages)
	if errors.Is(err, aiparse.ErrNothingVerified) {
		writeError(w, http.StatusUnprocessableEntity,
			"the AI's interpretation couldn't be verified against your document; please try again or paste the script as text")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI parsing is temporarily unavailable; please try again")
		return
	}

	writeJSON(w, http.StatusOK, importResponse{Elements: toCandidateElementDTOs(candidate.Elements)})
}

// pdfTextErrorMessage translates pdftext's sentinel errors into the
// specific, distinct user-facing messages the ingestion plan calls for --
// each names a different problem with a different fix, so they must not be
// collapsed into one generic message.
func pdfTextErrorMessage(err error) string {
	switch {
	case errors.Is(err, pdftext.ErrNotAPDF):
		return "the uploaded file is not a valid PDF"
	case errors.Is(err, pdftext.ErrNoTextLayer):
		return "this PDF doesn't appear to contain selectable text; scanned PDFs aren't supported"
	case errors.Is(err, pdftext.ErrTooManyPages):
		return fmt.Sprintf("this document has too many pages for this slice (max %d) -- try a shorter excerpt", pdftext.MaxPages)
	case errors.Is(err, pdftext.ErrTextTooLong):
		return "the extracted text is too long for this slice -- try a shorter excerpt"
	default:
		return "could not read this PDF"
	}
}

func toCandidateElementDTOs(elements []aiparse.CandidateElement) []candidateElementDTO {
	dtos := make([]candidateElementDTO, len(elements))
	for i, el := range elements {
		dtos[i] = candidateElementDTO{
			Kind:      string(el.Kind),
			Character: el.Character,
			Text:      el.Text,
			Page:      el.Page,
			Verified:  el.Verified,
		}
	}
	return dtos
}
