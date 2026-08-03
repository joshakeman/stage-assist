import { useState } from "react";
import { CompareForm } from "./features/compare/CompareForm";
import { ScriptImport } from "./features/import/ScriptImport";
import type { ConfirmedElement } from "./api/types";
import "./App.css";

function App() {
  const [importedElements, setImportedElements] = useState<ConfirmedElement[] | null>(null);

  return (
    <div className="app-page">
      <h1>Stage Assist</h1>
      <ScriptImport onConfirm={setImportedElements} />
      <CompareForm
        importedElements={importedElements}
        onClearImportedElements={() => setImportedElements(null)}
      />
    </div>
  );
}

export default App;
