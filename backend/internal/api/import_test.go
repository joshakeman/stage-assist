package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/api"
	"github.com/joshakeman/stage-assist/backend/internal/domain"
	"github.com/joshakeman/stage-assist/backend/internal/library"
)

func newTestStore(t *testing.T) *library.Store {
	t.Helper()
	store, err := library.NewStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func multipartUpload(t *testing.T, fieldName, filePath string) (*bytes.Buffer, string) {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, "upload.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

func doImport(t *testing.T, interpreter aiparse.ScriptInterpreter, fieldName, filePath string) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := multipartUpload(t, fieldName, filePath)
	req := httptest.NewRequest(http.MethodPost, "/api/scripts/import", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	api.NewMux(interpreter, newTestStore(t)).ServeHTTP(rec, req)
	return rec
}

const colonStyleFixture = "../pdftext/testdata/colon_style.pdf"

func TestHandleScriptImportHappyPath(t *testing.T) {
	fake := &aiparse.FakeInterpreter{Script: aiparse.CandidateScript{Elements: []aiparse.CandidateElement{
		{Kind: domain.KindDialogue, Character: "HAMLET", Text: "Who's there?", Page: 1, Verified: true},
		{Kind: domain.KindUnclassified, Text: "ACT ONE, SCENE ONE", Page: 1, Verified: false},
	}}}

	rec := doImport(t, fake, "file", colonStyleFixture)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !fake.Called {
		t.Error("interpreter was never called")
	}

	var resp struct {
		Elements []struct {
			Kind      string `json:"kind"`
			Character string `json:"character"`
			Text      string `json:"text"`
			Page      int    `json:"page"`
			Verified  bool   `json:"verified"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v; body = %s", err, rec.Body.String())
	}
	if len(resp.Elements) != 2 {
		t.Fatalf("got %d elements, want 2", len(resp.Elements))
	}
	if resp.Elements[0].Character != "HAMLET" || !resp.Elements[0].Verified {
		t.Errorf("element 0 = %+v", resp.Elements[0])
	}
	if resp.Elements[1].Kind != "unclassified" || resp.Elements[1].Verified {
		t.Errorf("element 1 = %+v, want unverified and not dropped", resp.Elements[1])
	}
}

func TestHandleScriptImportMissingFileField(t *testing.T) {
	fake := &aiparse.FakeInterpreter{}
	rec := doImport(t, fake, "wrong-field-name", colonStyleFixture)
	assertErrorStatus(t, rec, http.StatusBadRequest)
	if fake.Called {
		t.Error("interpreter should not be called when no file was uploaded")
	}
}

func TestHandleScriptImportRejectsNonPDF(t *testing.T) {
	fake := &aiparse.FakeInterpreter{}
	rec := doImport(t, fake, "file", "../pdftext/testdata/not_a_pdf.pdf")
	assertErrorStatus(t, rec, http.StatusBadRequest)
	if fake.Called {
		t.Error("interpreter should not be called for a file that isn't a PDF at all")
	}
}

func TestHandleScriptImportRejectsScannedPDF(t *testing.T) {
	fake := &aiparse.FakeInterpreter{}
	rec := doImport(t, fake, "file", "../pdftext/testdata/scanned_no_text_layer.pdf")
	assertErrorStatus(t, rec, http.StatusBadRequest)
	if fake.Called {
		t.Error("interpreter should not be called for a PDF with no text layer -- that's the whole point of rejecting it early")
	}
}

func TestHandleScriptImportReturnsUnprocessableWhenResponseWasTruncated(t *testing.T) {
	fake := &aiparse.FakeInterpreter{Err: aiparse.ErrResponseTruncated}
	rec := doImport(t, fake, "file", colonStyleFixture)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

func TestHandleScriptImportReturnsUnprocessableWhenNothingVerified(t *testing.T) {
	fake := &aiparse.FakeInterpreter{Err: aiparse.ErrNothingVerified}
	rec := doImport(t, fake, "file", colonStyleFixture)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
}

func TestHandleScriptImportReturnsBadGatewayOnInterpreterFailure(t *testing.T) {
	fake := &aiparse.FakeInterpreter{Err: errors.New("connection reset")}
	rec := doImport(t, fake, "file", colonStyleFixture)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

func TestHandleScriptImportRejectsUploadsOverTheSizeLimit(t *testing.T) {
	original := api.MaxUploadBytes
	api.MaxUploadBytes = 10 // smaller than any real fixture, no huge test file needed
	t.Cleanup(func() { api.MaxUploadBytes = original })

	fake := &aiparse.FakeInterpreter{}
	rec := doImport(t, fake, "file", colonStyleFixture)
	assertErrorStatus(t, rec, http.StatusBadRequest)
	if fake.Called {
		t.Error("interpreter should not be called for an oversized upload")
	}
}

func TestHandleScriptImportWrongMethodIsRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/scripts/import", nil)
	rec := httptest.NewRecorder()
	api.NewMux(&aiparse.FakeInterpreter{}, newTestStore(t)).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
