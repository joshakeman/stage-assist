import { useState } from "react";
import type { ConfirmedElement, SavedScriptSummary } from "../../api/types";
import { deleteSavedScript, getSavedScript } from "../../api/savedScriptsApi";
import { useSavedScripts } from "./useSavedScripts";
import "./library.css";

interface SavedScriptsListProps {
  onLoad: (elements: ConfirmedElement[], name: string) => void;
  onReReview: (elements: ConfirmedElement[]) => void;
}

export function SavedScriptsList({ onLoad, onReReview }: SavedScriptsListProps) {
  const { state, refresh } = useSavedScripts();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  async function handleLoad(summary: SavedScriptSummary) {
    setBusyId(summary.id);
    setActionError(null);
    try {
      const full = await getSavedScript(summary.id);
      onLoad(full.elements, full.name);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Could not load this script");
    } finally {
      setBusyId(null);
    }
  }

  async function handleReReview(summary: SavedScriptSummary) {
    setBusyId(summary.id);
    setActionError(null);
    try {
      const full = await getSavedScript(summary.id);
      onReReview(full.elements);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Could not load this script");
    } finally {
      setBusyId(null);
    }
  }

  async function handleDelete(summary: SavedScriptSummary) {
    setBusyId(summary.id);
    setActionError(null);
    try {
      await deleteSavedScript(summary.id);
      await refresh();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Could not delete this script");
    } finally {
      setBusyId(null);
    }
  }

  return (
    <section className="saved-scripts">
      <h2>Your saved scripts</h2>

      {state.status === "loading" && <p className="format-help">Loading saved scripts…</p>}

      {state.status === "error" && (
        <p className="compare-error" role="alert">
          {state.error}
        </p>
      )}

      {actionError && (
        <p className="compare-error" role="alert">
          {actionError}
        </p>
      )}

      {state.status === "success" && state.data.length === 0 && (
        <p className="format-help">
          Nothing saved yet — import a PDF below and save it once you've
          reviewed it, so you don't have to re-upload or pay for another AI
          pass next time.
        </p>
      )}

      {state.status === "success" && state.data.length > 0 && (
        <table className="saved-scripts-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Saved</th>
              <th>Lines</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {state.data.map((summary) => (
              <tr key={summary.id}>
                <td>{summary.name}</td>
                <td>{new Date(summary.createdAt).toLocaleString()}</td>
                <td>{summary.elementCount}</td>
                <td className="saved-scripts-actions">
                  <button
                    type="button"
                    onClick={() => handleLoad(summary)}
                    disabled={busyId === summary.id}
                  >
                    Load
                  </button>
                  <button
                    type="button"
                    className="script-import-secondary"
                    onClick={() => handleReReview(summary)}
                    disabled={busyId === summary.id}
                  >
                    Re-review
                  </button>
                  <button
                    type="button"
                    className="script-import-secondary"
                    onClick={() => handleDelete(summary)}
                    disabled={busyId === summary.id}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
