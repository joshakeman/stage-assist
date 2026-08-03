package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/joshakeman/stage-assist/backend/internal/aiparse"
	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

// server holds the dependencies handlers need beyond what a plain function
// can express -- currently just the AI interpreter the import endpoint
// calls. HandleCompare needs no such dependency (the transcript side is
// always plain text) so it stays a free function.
type server struct {
	interpreter aiparse.ScriptInterpreter
}

// NewMux builds the application's routing table. It exists so cmd/server
// and this package's tests share exactly one definition of the routes.
// interpreter is injected so tests can supply aiparse.FakeInterpreter
// instead of making real network calls.
func NewMux(interpreter aiparse.ScriptInterpreter) *http.ServeMux {
	s := &server{interpreter: interpreter}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/compare", HandleCompare)
	mux.HandleFunc("POST /api/scripts/import", s.handleScriptImport)
	return mux
}

// compareRequest's script side is a discriminated union rather than two
// independently-optional fields: {script?, scriptElements?} would have four
// reachable JSON states (neither/both/either set) for two valid ones,
// pushing "exactly one must be present" validation into the handler and
// risking silent frontend/backend drift given this project's hand-mirrored,
// uncodegenned types. A tagged union makes the invalid states
// unrepresentable instead.
type compareRequest struct {
	ScriptSource scriptSourceDTO `json:"scriptSource"`
	Transcript   string          `json:"transcript"`
	Character    string          `json:"character"`
}

// scriptSourceDTO is either raw pasted text (parsed with
// ParsePlainTextScript, exactly as before) or a list of user-confirmed
// elements from the PDF-import preview (converted directly into a
// domain.Script, bypassing text parsing entirely). Both branches converge
// on the same domain.Script -> ExtractCues -> Align call downstream --
// there is exactly one comparison code path, never two.
type scriptSourceDTO struct {
	Type     string                `json:"type"` // "raw" | "elements"
	Raw      string                `json:"raw,omitempty"`
	Elements []confirmedElementDTO `json:"elements,omitempty"`
}

// confirmedElementDTO is deliberately minimal compared to the import
// endpoint's candidateElementDTO: Verified/Page/SourceEvidence were only
// ever useful during review. Once a human has confirmed a row, they are no
// longer the authority -- the human is.
type confirmedElementDTO struct {
	Kind      string `json:"kind"`
	Character string `json:"character"`
	Text      string `json:"text"`
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

	transcript := strings.TrimSpace(req.Transcript)
	character := strings.TrimSpace(req.Character)

	if transcript == "" {
		writeError(w, http.StatusBadRequest, "transcript must not be empty")
		return
	}
	if character == "" {
		writeError(w, http.StatusBadRequest, "character must not be empty")
		return
	}

	scriptSide, err := scriptFromSource(req.ScriptSource)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	scriptCues := domain.ExtractCues(scriptSide, character)
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

// scriptFromSource builds a domain.Script from either branch of the
// scriptSource union.
func scriptFromSource(src scriptSourceDTO) (domain.Script, error) {
	switch src.Type {
	case "raw":
		raw := strings.TrimSpace(src.Raw)
		if raw == "" {
			return domain.Script{}, errors.New("scriptSource.raw must not be empty")
		}
		return domain.ParsePlainTextScript(raw), nil
	case "elements":
		return scriptFromConfirmedElements(src.Elements)
	default:
		return domain.Script{}, fmt.Errorf("scriptSource.type must be \"raw\" or \"elements\", got %q", src.Type)
	}
}

// scriptFromConfirmedElements converts user-confirmed import rows directly
// into a domain.Script. This lives here, in internal/api, not in
// internal/domain (which must never import ingestion-shaped types and stay
// the innermost, most stable package) and not in internal/aiparse (whose
// types this deliberately no longer resembles once import-time metadata is
// stripped). StartLine/EndLine are assigned sequentially since a confirmed
// element no longer maps to one single-format source position, and nothing
// downstream currently reads them beyond internal bookkeeping.
func scriptFromConfirmedElements(elements []confirmedElementDTO) (domain.Script, error) {
	if len(elements) == 0 {
		return domain.Script{}, errors.New("scriptSource.elements must not be empty")
	}

	scriptElements := make([]domain.ScriptElement, len(elements))
	for i, el := range elements {
		kind := domain.ElementKind(el.Kind)
		switch kind {
		case domain.KindDialogue, domain.KindDirection, domain.KindUnclassified:
		default:
			return domain.Script{}, fmt.Errorf("scriptSource.elements[%d]: invalid kind %q", i, el.Kind)
		}
		if kind == domain.KindDialogue && strings.TrimSpace(el.Character) == "" {
			return domain.Script{}, fmt.Errorf("scriptSource.elements[%d]: character is required when kind is dialogue", i)
		}
		if strings.TrimSpace(el.Text) == "" {
			return domain.Script{}, fmt.Errorf("scriptSource.elements[%d]: text must not be empty", i)
		}

		line := i + 1
		scriptElements[i] = domain.ScriptElement{
			Kind:      kind,
			Character: el.Character,
			Text:      el.Text,
			StartLine: line,
			EndLine:   line,
		}
	}
	return domain.Script{Elements: scriptElements}, nil
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
