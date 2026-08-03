import { useCallback, useState } from "react";
import type { ImportResponse } from "../../api/types";
import { importScript } from "../../api/importApi";

type ImportState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "success"; data: ImportResponse }
  | { status: "error"; error: string };

export function useImport() {
  const [state, setState] = useState<ImportState>({ status: "idle" });

  const run = useCallback(async (file: File) => {
    setState({ status: "loading" });
    try {
      const data = await importScript(file);
      setState({ status: "success", data });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Something went wrong";
      setState({ status: "error", error: message });
    }
  }, []);

  const reset = useCallback(() => setState({ status: "idle" }), []);

  return { state, run, reset };
}
