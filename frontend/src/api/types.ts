// Hand-written to mirror backend/internal/api/handlers.go's JSON DTOs
// (compareRequest / compareResponse / lineNoteDTO / wordDiffDTO). There is
// no codegen/OpenAPI step in this slice, so this file can drift from the Go
// contract — check it whenever handlers.go's DTOs change.

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

export interface CompareRequest {
  script: string;
  transcript: string;
  character: string;
}

export interface CompareResponse {
  character: string;
  notes: LineNote[];
}
