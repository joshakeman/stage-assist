// Hand-written to mirror backend/internal/api/handlers.go's JSON DTOs
// (compareRequest / compareResponse / lineNoteDTO / wordDiffDTO) and
// import.go's DTOs (candidateElementDTO / importResponse). There is no
// codegen/OpenAPI step in this slice, so this file can drift from the Go
// contract — check it whenever handlers.go's or import.go's DTOs change.

export type CueStatus = "exact" | "changed" | "missing" | "extra";

export type DiffOp = "equal" | "insert" | "delete";

export interface WordDiffSpan {
  op: DiffOp;
  text: string;
}

export interface LineNote {
  index: number;
  status: CueStatus;
  scriptText: string;
  spokenText: string;
  diff: WordDiffSpan[];
}

// Mirrors domain.ElementKind.
export type ElementKind = "dialogue" | "direction" | "unclassified";

// A candidate element as returned by the import endpoint, before the user
// has reviewed it -- still carries the trust signals (page, verified) a
// reviewer needs.
export interface CandidateElement {
  kind: ElementKind;
  character: string;
  text: string;
  page: number;
  verified: boolean;
}

export interface ImportResponse {
  elements: CandidateElement[];
}

// A user-confirmed row, ready to send to /api/compare. Deliberately
// smaller than CandidateElement: page/verified/sourceEvidence were only
// useful during review -- once a human has confirmed a row, they are the
// authority, not the import.
export interface ConfirmedElement {
  kind: ElementKind;
  character: string;
  text: string;
}

// A discriminated union, matching the backend's scriptSource exactly: the
// script side of a compare request is either pasted text (parsed the same
// way it always has been) or confirmed rows from a PDF import. There is no
// third, ambiguous state -- unlike two independently-optional fields would
// allow.
export type ScriptSource =
  | { type: "raw"; raw: string }
  | { type: "elements"; elements: ConfirmedElement[] };

export interface CompareRequest {
  scriptSource: ScriptSource;
  transcript: string;
  character: string;
}

export interface CompareResponse {
  character: string;
  notes: LineNote[];
}
