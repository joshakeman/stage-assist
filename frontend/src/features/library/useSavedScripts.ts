import { useCallback, useEffect, useState } from "react";
import type { SavedScriptSummary } from "../../api/types";
import { listSavedScripts } from "../../api/savedScriptsApi";

type SavedScriptsState =
  | { status: "loading" }
  | { status: "success"; data: SavedScriptSummary[] }
  | { status: "error"; error: string };

function toErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : "Something went wrong";
}

// Fetches the saved-script list on mount, and again whenever refresh() is
// called (e.g. after a Save or Delete elsewhere). No setState call happens
// synchronously within the effect body itself -- the fetch's outcome is
// only ever applied inside its own .then()/.catch() callbacks, which is
// the "calling setState in a callback function when external state
// changes" pattern React's effect guidance recommends, as opposed to a
// direct synchronous setState in the effect (which react-hooks/
// set-state-in-effect flags, and which refresh()'s own synchronous
// setState is fine to do -- it's only ever called from event handlers,
// never from inside this effect).
export function useSavedScripts() {
  const [state, setState] = useState<SavedScriptsState>({ status: "loading" });
  const [refreshToken, setRefreshToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    listSavedScripts().then(
      (data) => {
        if (!cancelled) setState({ status: "success", data });
      },
      (err: unknown) => {
        if (!cancelled) setState({ status: "error", error: toErrorMessage(err) });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [refreshToken]);

  const refresh = useCallback(() => {
    setState({ status: "loading" });
    setRefreshToken((t) => t + 1);
  }, []);

  return { state, refresh };
}
