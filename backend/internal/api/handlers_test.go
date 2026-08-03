package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/api"
)

func doCompare(t *testing.T, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/compare", strings.NewReader(body))
	rec := httptest.NewRecorder()
	api.NewMux(&aiparse.FakeInterpreter{}, newTestStore(t)).ServeHTTP(rec, req)
	return rec
}

func TestHandleCompareExactMatch(t *testing.T) {
	body := `{"scriptSource":{"type":"raw","raw":"HAMLET: To be or not to be\n"},"transcript":"HAMLET: To be or not to be\n","character":"HAMLET"}`
	rec := doCompare(t, http.MethodPost, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Character string `json:"character"`
		Notes     []struct {
			Status string `json:"status"`
		} `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v; body = %s", err, rec.Body.String())
	}
	if resp.Character != "HAMLET" {
		t.Errorf("character = %q, want HAMLET", resp.Character)
	}
	if len(resp.Notes) != 1 || resp.Notes[0].Status != "exact" {
		t.Errorf("notes = %+v, want one exact note", resp.Notes)
	}
}

func TestHandleCompareMalformedJSON(t *testing.T) {
	rec := doCompare(t, http.MethodPost, `{not valid json`)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}

func TestHandleCompareMissingCharacterField(t *testing.T) {
	body := `{"scriptSource":{"type":"raw","raw":"HAMLET: To be or not to be\n"},"transcript":"HAMLET: To be or not to be\n"}`
	rec := doCompare(t, http.MethodPost, body)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}

func TestHandleCompareCharacterAbsentFromScriptIsRejected(t *testing.T) {
	body := `{"scriptSource":{"type":"raw","raw":"OPHELIA: My lord\n"},"transcript":"HAMLET: To be or not to be\n","character":"HAMLET"}`
	rec := doCompare(t, http.MethodPost, body)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}

func TestHandleCompareCharacterAbsentFromTranscriptIsValid(t *testing.T) {
	body := `{"scriptSource":{"type":"raw","raw":"HAMLET: To be or not to be\n"},"transcript":"OPHELIA: My lord\n","character":"HAMLET"}`
	rec := doCompare(t, http.MethodPost, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Notes []struct {
			Status string `json:"status"`
		} `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v; body = %s", err, rec.Body.String())
	}
	if len(resp.Notes) != 1 || resp.Notes[0].Status != "missing" {
		t.Errorf("notes = %+v, want one missing note, not an error", resp.Notes)
	}
}

func TestHandleCompareWrongMethodIsRejected(t *testing.T) {
	rec := doCompare(t, http.MethodGet, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// The other, newer way to produce a script side: confirmed elements from a
// PDF import, rather than pasted text. This must converge on exactly the
// same Align call as the "raw" branch -- same response shape, same status
// derivation.
func TestHandleCompareWithConfirmedElements(t *testing.T) {
	body := `{"scriptSource":{"type":"elements","elements":[
		{"kind":"dialogue","character":"HAMLET","text":"To be or not to be"},
		{"kind":"direction","character":"","text":"He pauses."}
	]},"transcript":"HAMLET: To be or not to be\n","character":"HAMLET"}`
	rec := doCompare(t, http.MethodPost, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Notes []struct {
			Status string `json:"status"`
		} `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v; body = %s", err, rec.Body.String())
	}
	if len(resp.Notes) != 1 || resp.Notes[0].Status != "exact" {
		t.Errorf("notes = %+v, want one exact note (the direction element must not become a cue)", resp.Notes)
	}
}

func TestHandleCompareRejectsUnknownScriptSourceType(t *testing.T) {
	body := `{"scriptSource":{"type":"pdf-url","raw":"https://example.com/script.pdf"},"transcript":"HAMLET: hi\n","character":"HAMLET"}`
	rec := doCompare(t, http.MethodPost, body)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}

func TestHandleCompareRejectsInvalidElementKind(t *testing.T) {
	body := `{"scriptSource":{"type":"elements","elements":[{"kind":"soliloquy","character":"HAMLET","text":"hi"}]},"transcript":"HAMLET: hi\n","character":"HAMLET"}`
	rec := doCompare(t, http.MethodPost, body)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}

func TestHandleCompareRejectsDialogueElementWithoutCharacter(t *testing.T) {
	body := `{"scriptSource":{"type":"elements","elements":[{"kind":"dialogue","character":"","text":"hi"}]},"transcript":"HAMLET: hi\n","character":"HAMLET"}`
	rec := doCompare(t, http.MethodPost, body)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}

func assertErrorStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON error response: %v; body = %s", err, rec.Body.String())
	}
	if resp.Error == "" {
		t.Errorf("error message is empty")
	}
}
