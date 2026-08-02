import type { LineNote } from "../../api/types";
import { WordDiffText } from "./WordDiffText";

const STATUS_LABEL: Record<LineNote["status"], string> = {
  exact: "Exact",
  changed: "Changed",
  missing: "Dropped line",
  extra: "Added line",
};

interface LineNoteRowProps {
  note: LineNote;
}

export function LineNoteRow({ note }: LineNoteRowProps) {
  // Only "changed" has a real word-level diff to highlight. An "exact" cue's
  // diff is, by construction, entirely "equal" spans (cue-level equality
  // requires every token to normalize-match) -- and WordDiff's "equal" spans
  // carry only the script's surface text, so rendering an exact cue through
  // the diff would show the script's punctuation/wording on the spoken side
  // too. Render each column's own text directly instead; there's nothing to
  // diff.
  const showDiff = note.status === "changed";

  return (
    <tr className={`line-note-row line-note-row--${note.status}`}>
      <td className="line-note-status">
        <span className={`status-badge status-badge--${note.status}`}>
          {STATUS_LABEL[note.status]}
        </span>
      </td>
      <td className="line-note-script">
        {note.status === "extra" ? (
          <span className="word-diff-empty">— not in script —</span>
        ) : showDiff ? (
          <WordDiffText spans={note.diff} side="script" />
        ) : (
          note.scriptText
        )}
      </td>
      <td className="line-note-spoken">
        {note.status === "missing" ? (
          <span className="word-diff-empty">— line not spoken —</span>
        ) : showDiff ? (
          <WordDiffText spans={note.diff} side="spoken" />
        ) : (
          note.spokenText
        )}
      </td>
    </tr>
  );
}
