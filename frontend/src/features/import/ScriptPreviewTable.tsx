import type { CandidateElement, ElementKind } from "../../api/types";

export interface EditableRow extends CandidateElement {
  id: number;
}

type RowPatch = Partial<Pick<EditableRow, "kind" | "character" | "text">>;

interface ScriptPreviewTableProps {
  rows: EditableRow[];
  onChangeRow: (id: number, patch: RowPatch) => void;
  onDeleteRow: (id: number) => void;
}

const KIND_OPTIONS: { value: ElementKind; label: string }[] = [
  { value: "dialogue", label: "Dialogue" },
  { value: "direction", label: "Direction" },
  { value: "unclassified", label: "Unclassified" },
];

export function ScriptPreviewTable({ rows, onChangeRow, onDeleteRow }: ScriptPreviewTableProps) {
  return (
    <table className="script-preview-table">
      <thead>
        <tr>
          <th>Kind</th>
          <th>Character</th>
          <th>Text</th>
          <th>Page</th>
          <th>Verified</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.id} className={row.verified ? undefined : "script-preview-row--unverified"}>
            <td>
              <select
                value={row.kind}
                onChange={(e) => onChangeRow(row.id, { kind: e.target.value as ElementKind })}
              >
                {KIND_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </td>
            <td>
              <input
                type="text"
                value={row.character}
                disabled={row.kind !== "dialogue"}
                onChange={(e) => onChangeRow(row.id, { character: e.target.value })}
                placeholder={row.kind === "dialogue" ? "CHARACTER" : "—"}
              />
            </td>
            <td>
              <textarea
                className="script-preview-text"
                value={row.text}
                onChange={(e) => onChangeRow(row.id, { text: e.target.value })}
                rows={2}
              />
            </td>
            <td className="script-preview-page">{row.page > 0 ? `p.${row.page}` : "page unknown"}</td>
            <td>
              {!row.verified && (
                <span className="script-preview-flag" title="Could not be verified against the source document">
                  unverified
                </span>
              )}
            </td>
            <td>
              <button type="button" onClick={() => onDeleteRow(row.id)} aria-label="Delete row">
                Delete
              </button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
