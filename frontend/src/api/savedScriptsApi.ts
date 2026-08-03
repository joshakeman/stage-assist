import type { ConfirmedElement, SavedScript, SavedScriptSummary } from "./types";
import { ApiError } from "./compareApi";

async function throwIfNotOk(res: Response): Promise<void> {
  if (res.ok) return;
  const body: unknown = await res.json().catch(() => null);
  const message =
    body && typeof body === "object" && "error" in body &&
    typeof body.error === "string"
      ? body.error
      : `Request failed with status ${res.status}`;
  throw new ApiError(res.status, message);
}

export async function listSavedScripts(): Promise<SavedScriptSummary[]> {
  const res = await fetch("/api/scripts/saved");
  await throwIfNotOk(res);
  return res.json() as Promise<SavedScriptSummary[]>;
}

export async function saveScript(
  name: string,
  elements: ConfirmedElement[],
): Promise<SavedScript> {
  const res = await fetch("/api/scripts/saved", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, elements }),
  });
  await throwIfNotOk(res);
  return res.json() as Promise<SavedScript>;
}

export async function getSavedScript(id: string): Promise<SavedScript> {
  const res = await fetch(`/api/scripts/saved/${encodeURIComponent(id)}`);
  await throwIfNotOk(res);
  return res.json() as Promise<SavedScript>;
}

export async function deleteSavedScript(id: string): Promise<void> {
  const res = await fetch(`/api/scripts/saved/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  await throwIfNotOk(res);
}
