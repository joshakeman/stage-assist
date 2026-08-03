import { useMemo, useState, type FormEvent } from "react";
import type { ConfirmedElement, ScriptSource } from "../../api/types";
import { LineNotesList } from "./LineNotesList";
import { useCompare } from "./useCompare";
import "./compare.css";

interface CompareFormProps {
  importedElements: ConfirmedElement[] | null;
  onClearImportedElements: () => void;
}

export function CompareForm({ importedElements, onClearImportedElements }: CompareFormProps) {
  const [script, setScript] = useState("");
  const [transcript, setTranscript] = useState("");
  const [character, setCharacter] = useState("");
  const { state, run } = useCompare();

  // Offered as suggestions for the character field, not a hard restriction
  // -- closes a real mismatch risk (e.g. "Romeo" imported vs "ROMEO" typed)
  // without forcing the user to give up free text.
  const importedCharacters = useMemo(() => {
    if (!importedElements) {
      return [];
    }
    const names = importedElements
      .filter((el) => el.kind === "dialogue" && el.character.trim() !== "")
      .map((el) => el.character);
    return Array.from(new Set(names));
  }, [importedElements]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const scriptSource: ScriptSource = importedElements
      ? { type: "elements", elements: importedElements }
      : { type: "raw", raw: script };
    run({ scriptSource, transcript, character });
  }

  return (
    <section className="compare-section">
      <h2>Compare</h2>

      <form className="compare-form" onSubmit={handleSubmit}>
        {importedElements ? (
          <div className="compare-field">
            Script
            <p className="compare-imported-summary">
              Using {importedElements.length} confirmed line
              {importedElements.length === 1 ? "" : "s"} from the PDF import.{" "}
              <button type="button" className="link-button" onClick={onClearImportedElements}>
                Clear and paste text instead
              </button>
            </p>
          </div>
        ) : (
          <>
            <p className="format-help">
              One cue per line, formatted as <code>CHARACTER: dialogue text</code>
              . Lines without this format (scene headers, stage directions) are
              ignored.
            </p>

            <label className="compare-field">
              Script
              <textarea
                value={script}
                onChange={(e) => setScript(e.target.value)}
                placeholder="HAMLET: To be or not to be"
                rows={10}
              />
            </label>
          </>
        )}

        <label className="compare-field">
          Transcript
          <textarea
            value={transcript}
            onChange={(e) => setTranscript(e.target.value)}
            placeholder="HAMLET: To be or not to be"
            rows={10}
          />
        </label>

        <label className="compare-field compare-field--character">
          Character
          <input
            type="text"
            list={importedCharacters.length > 0 ? "imported-character-options" : undefined}
            value={character}
            onChange={(e) => setCharacter(e.target.value)}
            placeholder="HAMLET"
          />
          {importedCharacters.length > 0 && (
            <datalist id="imported-character-options">
              {importedCharacters.map((name) => (
                <option key={name} value={name} />
              ))}
            </datalist>
          )}
        </label>

        <button type="submit" disabled={state.status === "loading"}>
          {state.status === "loading" ? "Comparing…" : "Compare"}
        </button>
      </form>

      {state.status === "error" && (
        <p className="compare-error" role="alert">
          {state.error}
        </p>
      )}

      {state.status === "success" && (
        <section className="compare-results">
          <h2>Line notes for {state.data.character}</h2>
          <LineNotesList notes={state.data.notes} />
        </section>
      )}
    </section>
  );
}
