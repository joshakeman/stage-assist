import type { LineNote } from "../../api/types";
import { LineNoteRow } from "./LineNoteRow";

interface LineNotesListProps {
  notes: LineNote[];
}

export function LineNotesList({ notes }: LineNotesListProps) {
  return (
    <table className="line-notes-list">
      <thead>
        <tr>
          <th>Status</th>
          <th>Script</th>
          <th>Spoken</th>
        </tr>
      </thead>
      <tbody>
        {notes.map((note) => (
          <LineNoteRow key={note.index} note={note} />
        ))}
      </tbody>
    </table>
  );
}
