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

func doLibraryRequest(t *testing.T, mux http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestSavedScriptsListStartsEmpty(t *testing.T) {
	mux := api.NewMux(&aiparse.FakeInterpreter{}, newTestStore(t))
	rec := doLibraryRequest(t, mux, http.MethodGet, "/api/scripts/saved", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got []struct{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; body = %s", err, rec.Body.String())
	}
	if len(got) != 0 {
		t.Errorf("got %d saved scripts, want 0", len(got))
	}
}

func TestSaveListGetDeleteRoundTrip(t *testing.T) {
	mux := api.NewMux(&aiparse.FakeInterpreter{}, newTestStore(t))

	saveBody := `{"name":"Hamlet Act 1","elements":[{"kind":"dialogue","character":"HAMLET","text":"Who's there?"}]}`
	saveRec := doLibraryRequest(t, mux, http.MethodPost, "/api/scripts/saved", saveBody)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d; body = %s", saveRec.Code, http.StatusOK, saveRec.Body.String())
	}

	var saved struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Elements []struct {
			Kind      string `json:"kind"`
			Character string `json:"character"`
			Text      string `json:"text"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("invalid JSON: %v; body = %s", err, saveRec.Body.String())
	}
	if saved.ID == "" || saved.Name != "Hamlet Act 1" || len(saved.Elements) != 1 {
		t.Fatalf("saved = %+v, unexpected shape", saved)
	}

	listRec := doLibraryRequest(t, mux, http.MethodGet, "/api/scripts/saved", "")
	var list []struct {
		ID           string `json:"id"`
		ElementCount int    `json:"elementCount"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("invalid JSON: %v; body = %s", err, listRec.Body.String())
	}
	if len(list) != 1 || list[0].ID != saved.ID || list[0].ElementCount != 1 {
		t.Fatalf("list = %+v, want one entry matching id=%s elementCount=1", list, saved.ID)
	}

	getRec := doLibraryRequest(t, mux, http.MethodGet, "/api/scripts/saved/"+saved.ID, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body = %s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	deleteRec := doLibraryRequest(t, mux, http.MethodDelete, "/api/scripts/saved/"+saved.ID, "")
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d; body = %s", deleteRec.Code, http.StatusNoContent, deleteRec.Body.String())
	}

	getAfterDeleteRec := doLibraryRequest(t, mux, http.MethodGet, "/api/scripts/saved/"+saved.ID, "")
	assertErrorStatus(t, getAfterDeleteRec, http.StatusNotFound)
}

func TestGetAndDeleteUnknownSavedScriptReturnNotFound(t *testing.T) {
	mux := api.NewMux(&aiparse.FakeInterpreter{}, newTestStore(t))
	assertErrorStatus(t, doLibraryRequest(t, mux, http.MethodGet, "/api/scripts/saved/does-not-exist", ""), http.StatusNotFound)
	assertErrorStatus(t, doLibraryRequest(t, mux, http.MethodDelete, "/api/scripts/saved/does-not-exist", ""), http.StatusNotFound)
}

// TestSaveScriptRejectsInvalidElements confirms handleSaveScript actually
// runs scriptFromConfirmedElements's validation, not just its own checks --
// a dialogue element with no character is the same invalid shape
// /api/compare already rejects.
func TestSaveScriptRejectsInvalidElements(t *testing.T) {
	mux := api.NewMux(&aiparse.FakeInterpreter{}, newTestStore(t))
	body := `{"name":"Bad","elements":[{"kind":"dialogue","character":"","text":"hello"}]}`
	rec := doLibraryRequest(t, mux, http.MethodPost, "/api/scripts/saved", body)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}

func TestSaveScriptRejectsEmptyName(t *testing.T) {
	mux := api.NewMux(&aiparse.FakeInterpreter{}, newTestStore(t))
	body := `{"name":"","elements":[{"kind":"dialogue","character":"HAMLET","text":"hello"}]}`
	rec := doLibraryRequest(t, mux, http.MethodPost, "/api/scripts/saved", body)
	assertErrorStatus(t, rec, http.StatusBadRequest)
}
