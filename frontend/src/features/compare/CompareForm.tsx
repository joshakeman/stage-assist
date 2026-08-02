import { useState, type FormEvent } from "react";
import { LineNotesList } from "./LineNotesList";
import { useCompare } from "./useCompare";
import "./compare.css";

export function CompareForm() {
  const [script, setScript] = useState("");
  const [transcript, setTranscript] = useState("");
  const [character, setCharacter] = useState("");
  const { state, run } = useCompare();

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    run({ script, transcript, character });
  }

  return (
    <div className="compare-page">
      <h1>Stage Assist</h1>

      <form className="compare-form" onSubmit={handleSubmit}>
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
            value={character}
            onChange={(e) => setCharacter(e.target.value)}
            placeholder="HAMLET"
          />
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
    </div>
  );
}
