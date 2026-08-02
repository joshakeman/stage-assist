package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

// NewMux builds the application's routing table. It exists so cmd/server
// and this package's tests share exactly one definition of the routes.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/compare", HandleCompare)
	return mux
}

type compareRequest struct {
	Script     string `json:"script"`
	Transcript string `json:"transcript"`
	Character  string `json:"character"`
}

type compareResponse struct {
	Character string        `json:"character"`
	Notes     []lineNoteDTO `json:"notes"`
}

type lineNoteDTO struct {
	Index      int           `json:"index"`
	Status     string        `json:"status"`
	ScriptText string        `json:"scriptText"`
	SpokenText string        `json:"spokenText"`
	Diff       []wordDiffDTO `json:"diff"`
}

type wordDiffDTO struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// HandleCompare parses a script and transcript, extracts one character's
// cues from each, and returns the line notes produced by aligning them.
//
// A character with no cues in the script is rejected as a validation error
// (almost certainly a mistyped or absent name — there's nothing to compare
// against). A character with no cues in the transcript is NOT an error: it's
// a valid, meaningful result meaning the actor never said any of their
// lines, and Align naturally reports every script cue as missing.
func HandleCompare(w http.ResponseWriter, r *http.Request) {
	var req compareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	script := strings.TrimSpace(req.Script)
	transcript := strings.TrimSpace(req.Transcript)
	character := strings.TrimSpace(req.Character)

	if script == "" {
		writeError(w, http.StatusBadRequest, "script must not be empty")
		return
	}
	if transcript == "" {
		writeError(w, http.StatusBadRequest, "transcript must not be empty")
		return
	}
	if character == "" {
		writeError(w, http.StatusBadRequest, "character must not be empty")
		return
	}

	scriptCues := domain.ExtractCues(domain.ParsePlainTextScript(script), character)
	if len(scriptCues) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("character %q has no lines in the script", character))
		return
	}
	transcriptCues := domain.ExtractCues(domain.ParsePlainTextScript(transcript), character)

	notes := domain.Align(scriptCues, transcriptCues)

	writeJSON(w, http.StatusOK, compareResponse{
		Character: character,
		Notes:     toLineNoteDTOs(notes),
	})
}

func toLineNoteDTOs(notes []domain.LineNote) []lineNoteDTO {
	dtos := make([]lineNoteDTO, len(notes))
	for i, n := range notes {
		dtos[i] = lineNoteDTO{
			Index:      n.Index,
			Status:     string(n.Status),
			ScriptText: n.ScriptText,
			SpokenText: n.SpokenText,
			Diff:       toWordDiffDTOs(n.Diff),
		}
	}
	return dtos
}

func toWordDiffDTOs(spans []domain.WordDiffSpan) []wordDiffDTO {
	dtos := make([]wordDiffDTO, len(spans))
	for i, s := range spans {
		dtos[i] = wordDiffDTO{Op: string(s.Op), Text: s.Text}
	}
	return dtos
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
