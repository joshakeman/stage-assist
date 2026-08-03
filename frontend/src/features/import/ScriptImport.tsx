import { useState } from "react";
import type { ConfirmedElement, ImportResponse } from "../../api/types";
import { PdfUploadForm } from "./PdfUploadForm";
import { ScriptPreviewTable, type EditableRow } from "./ScriptPreviewTable";
import { useImport } from "./useImport";
import { useSaveScript } from "./useSaveScript";
import "./import.css";

export interface ReReviewRequest {
  requestId: number;
  elements: ConfirmedElement[];
}

interface ScriptImportProps {
  onConfirm: (elements: ConfirmedElement[]) => void;
  reReviewRequest?: ReReviewRequest | null;
}

export function ScriptImport({ onConfirm, reReviewRequest }: ScriptImportProps) {
  const { state, run, reset } = useImport();
  const [rows, setRows] = useState<EditableRow[] | null>(null);
  // Tracks which successful import response rows[] was last seeded from, so
  // a new import can re-seed rows while local edits to the current one
  // don't get clobbered on every render. Deriving state during render like
  // this (rather than in a useEffect) is the pattern React recommends for
  // "reset local state when an external value changes."
  const [seenData, setSeenData] = useState<ImportResponse | null>(null);

  if (state.status === "success" && state.data !== seenData) {
    setSeenData(state.data);
    setRows(state.data.elements.map((el, i) => ({ ...el, id: i })));
  }

  // Same derived-during-render pattern, keyed by requestId rather than by
  // the elements themselves, so re-requesting a review of the very same
  // saved script twice in a row is still detected as a fresh request.
  const [seenReReviewRequestId, setSeenReReviewRequestId] = useState<number | null>(null);

  if (reReviewRequest && reReviewRequest.requestId !== seenReReviewRequestId) {
    setSeenReReviewRequestId(reReviewRequest.requestId);
    // page/verified never really applied to a saved script's reloaded
    // content -- those signals only ever meant "Go's grounding check
    // passed against the original PDF," which didn't run again here.
    // page: 0 renders as "page unknown"; verified: true just suppresses
    // the "unverified" flag, since a human already confirmed these rows
    // once before saving them.
    setRows(reReviewRequest.elements.map((el, i) => ({ ...el, page: 0, verified: true, id: i })));
    reset();
  }

  const [saveName, setSaveName] = useState("");
  const { state: saveState, run: runSave } = useSaveScript();

  function handleChangeRow(
    id: number,
    patch: Partial<Pick<EditableRow, "kind" | "character" | "text">>,
  ) {
    setRows((prev) => prev?.map((row) => (row.id === id ? { ...row, ...patch } : row)) ?? null);
  }

  function handleDeleteRow(id: number) {
    setRows((prev) => prev?.filter((row) => row.id !== id) ?? null);
  }

  function handleStartOver() {
    setRows(null);
    reset();
  }

  function handleConfirm() {
    if (!rows || rows.length === 0) {
      return;
    }
    onConfirm(rows.map(({ kind, character, text }) => ({ kind, character, text })));
  }

  function handleSave() {
    if (!rows || rows.length === 0 || saveName.trim() === "") {
      return;
    }
    runSave(saveName.trim(), rows.map(({ kind, character, text }) => ({ kind, character, text })));
  }

  return (
    <section className="script-import">
      <h2>Import script from PDF</h2>
      <p className="format-help">
        Upload a short script excerpt (up to 15 pages) with a real text
        layer — scanned PDFs aren't supported. Claude proposes a structure;
        review and edit it below before using it in a comparison.
      </p>

      {rows === null && <PdfUploadForm onSubmit={run} loading={state.status === "loading"} />}

      {state.status === "error" && (
        <p className="compare-error" role="alert">
          {state.error}
        </p>
      )}

      {rows !== null && (
        <>
          <p className="script-import-warning">
            This import is only kept in memory — reloading the page will
            lose it, unless you save it to your script library below.
          </p>
          <ScriptPreviewTable rows={rows} onChangeRow={handleChangeRow} onDeleteRow={handleDeleteRow} />
          <div className="script-import-actions">
            <button type="button" onClick={handleConfirm} disabled={rows.length === 0}>
              Use these {rows.length} line{rows.length === 1 ? "" : "s"}
            </button>
            <button type="button" className="script-import-secondary" onClick={handleStartOver}>
              Discard and upload a different file
            </button>
          </div>
          <div className="script-import-save">
            <input
              type="text"
              value={saveName}
              onChange={(e) => setSaveName(e.target.value)}
              placeholder="Name this script to save it for later"
              disabled={saveState.status === "saving"}
            />
            <button
              type="button"
              className="script-import-secondary"
              onClick={handleSave}
              disabled={saveState.status === "saving" || saveName.trim() === ""}
            >
              Save
            </button>
            {saveState.status === "success" && (
              <span className="script-import-save-confirmation">
                Saved as "{saveState.data.name}".
              </span>
            )}
            {saveState.status === "error" && (
              <span className="compare-error" role="alert">
                {saveState.error}
              </span>
            )}
          </div>
        </>
      )}
    </section>
  );
}
