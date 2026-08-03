import { useRef, useState } from "react";
import { CompareForm } from "./features/compare/CompareForm";
import { ScriptImport, type ReReviewRequest } from "./features/import/ScriptImport";
import { SavedScriptsList } from "./features/library/SavedScriptsList";
import type { ConfirmedElement } from "./api/types";
import "./App.css";

function App() {
  const [importedElements, setImportedElements] = useState<ConfirmedElement[] | null>(null);
  const [loadedScriptName, setLoadedScriptName] = useState<string | null>(null);
  const [reReviewRequest, setReReviewRequest] = useState<ReReviewRequest | null>(null);
  const nextReReviewRequestId = useRef(1);

  function handleConfirm(elements: ConfirmedElement[]) {
    setImportedElements(elements);
    setLoadedScriptName(null);
  }

  function handleLoadSavedScript(elements: ConfirmedElement[], name: string) {
    setImportedElements(elements);
    setLoadedScriptName(name);
  }

  function handleReReview(elements: ConfirmedElement[]) {
    setReReviewRequest({ requestId: nextReReviewRequestId.current++, elements });
  }

  function handleClearImportedElements() {
    setImportedElements(null);
    setLoadedScriptName(null);
  }

  return (
    <div className="app-page">
      <h1>Stage Assist</h1>
      <SavedScriptsList onLoad={handleLoadSavedScript} onReReview={handleReReview} />
      <ScriptImport onConfirm={handleConfirm} reReviewRequest={reReviewRequest} />
      <CompareForm
        importedElements={importedElements}
        loadedScriptName={loadedScriptName}
        onClearImportedElements={handleClearImportedElements}
        onReReview={importedElements ? () => handleReReview(importedElements) : undefined}
      />
    </div>
  );
}

export default App;
