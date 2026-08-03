import { useState, type FormEvent } from "react";

interface PdfUploadFormProps {
  onSubmit: (file: File) => void;
  loading: boolean;
}

export function PdfUploadForm({ onSubmit, loading }: PdfUploadFormProps) {
  const [file, setFile] = useState<File | null>(null);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (file) {
      onSubmit(file);
    }
  }

  return (
    <form className="pdf-upload-form" onSubmit={handleSubmit}>
      <label className="compare-field">
        Script PDF
        <input
          type="file"
          accept="application/pdf"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
        />
      </label>
      <button type="submit" disabled={!file || loading}>
        {loading ? "Reading and interpreting…" : "Upload and interpret"}
      </button>
    </form>
  );
}
