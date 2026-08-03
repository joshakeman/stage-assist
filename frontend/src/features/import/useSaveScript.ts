import { useCallback, useState } from "react";
import type { ConfirmedElement, SavedScript } from "../../api/types";
import { saveScript } from "../../api/savedScriptsApi";

type SaveScriptState =
  | { status: "idle" }
  | { status: "saving" }
  | { status: "success"; data: SavedScript }
  | { status: "error"; error: string };

export function useSaveScript() {
  const [state, setState] = useState<SaveScriptState>({ status: "idle" });

  const run = useCallback(async (name: string, elements: ConfirmedElement[]) => {
    setState({ status: "saving" });
    try {
      const data = await saveScript(name, elements);
      setState({ status: "success", data });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Something went wrong";
      setState({ status: "error", error: message });
    }
  }, []);

  const reset = useCallback(() => setState({ status: "idle" }), []);

  return { state, run, reset };
}
