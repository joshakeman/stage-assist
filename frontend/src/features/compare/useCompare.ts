import { useCallback, useState } from "react";
import type { CompareRequest, CompareResponse } from "../../api/types";
import { compareScripts } from "../../api/compareApi";

type CompareState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "success"; data: CompareResponse }
  | { status: "error"; error: string };

export function useCompare() {
  const [state, setState] = useState<CompareState>({ status: "idle" });

  const run = useCallback(async (req: CompareRequest) => {
    setState({ status: "loading" });
    try {
      const data = await compareScripts(req);
      setState({ status: "success", data });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Something went wrong";
      setState({ status: "error", error: message });
    }
  }, []);

  return { state, run };
}
