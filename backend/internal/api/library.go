package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/joshakeman/stage-assist/backend/internal/library"
)

// savedScriptSummaryDTO is the lightweight shape for listing saved
// scripts -- no Elements, so browsing the library doesn't require
// shipping every saved script's full content over the wire.
type savedScriptSummaryDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CreatedAt    string `json:"createdAt"`
	ElementCount int    `json:"elementCount"`
}

// savedScriptDTO is the full shape for one saved script -- returned by
// save and get, used for both loading straight into a comparison and
// re-review.
type savedScriptDTO struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	CreatedAt string                `json:"createdAt"`
	Elements  []confirmedElementDTO `json:"elements"`
}

type saveScriptRequest struct {
	Name     string                `json:"name"`
	Elements []confirmedElementDTO `json:"elements"`
}

// handleListSavedScripts returns every saved script, newest first.
func (s *server) handleListSavedScripts(w http.ResponseWriter, r *http.Request) {
	scripts, err := s.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list saved scripts")
		return
	}

	summaries := make([]savedScriptSummaryDTO, len(scripts))
	for i, sc := range scripts {
		summaries[i] = savedScriptSummaryDTO{
			ID:           sc.ID,
			Name:         sc.Name,
			CreatedAt:    sc.CreatedAt.Format(time.RFC3339),
			ElementCount: len(sc.Elements),
		}
	}
	writeJSON(w, http.StatusOK, summaries)
}

// handleSaveScript saves a named, already-confirmed script. It reuses
// scriptFromConfirmedElements purely for its validation (kind must be
// valid, dialogue requires a character, text must be non-empty) --
// discarding the domain.Script it builds -- so nothing invalid can be
// saved to reappear later as a broken reload.
func (s *server) handleSaveScript(w http.ResponseWriter, r *http.Request) {
	var req saveScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name must not be empty")
		return
	}
	if _, err := scriptFromConfirmedElements(req.Elements); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	elements := make([]library.Element, len(req.Elements))
	for i, el := range req.Elements {
		elements[i] = library.Element{Kind: el.Kind, Character: el.Character, Text: el.Text}
	}

	saved, err := s.store.Save(name, elements)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save script")
		return
	}
	writeJSON(w, http.StatusOK, toSavedScriptDTO(saved))
}

// handleGetSavedScript returns one saved script's full content, used to
// load it straight into a comparison or to re-open it for review.
func (s *server) handleGetSavedScript(w http.ResponseWriter, r *http.Request) {
	saved, err := s.store.Get(r.PathValue("id"))
	if errors.Is(err, library.ErrNotFound) {
		writeError(w, http.StatusNotFound, "saved script not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not get saved script")
		return
	}
	writeJSON(w, http.StatusOK, toSavedScriptDTO(saved))
}

// handleDeleteSavedScript removes one saved script.
func (s *server) handleDeleteSavedScript(w http.ResponseWriter, r *http.Request) {
	err := s.store.Delete(r.PathValue("id"))
	if errors.Is(err, library.ErrNotFound) {
		writeError(w, http.StatusNotFound, "saved script not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete saved script")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toSavedScriptDTO(saved library.SavedScript) savedScriptDTO {
	elements := make([]confirmedElementDTO, len(saved.Elements))
	for i, el := range saved.Elements {
		elements[i] = confirmedElementDTO{Kind: el.Kind, Character: el.Character, Text: el.Text}
	}
	return savedScriptDTO{
		ID:        saved.ID,
		Name:      saved.Name,
		CreatedAt: saved.CreatedAt.Format(time.RFC3339),
		Elements:  elements,
	}
}
