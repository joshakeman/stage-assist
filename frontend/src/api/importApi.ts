import type { ImportResponse } from "./types";
import { ApiError } from "./compareApi";

export async function importScript(file: File): Promise<ImportResponse> {
  const formData = new FormData();
  formData.append("file", file);

  const res = await fetch("/api/scripts/import", {
    method: "POST",
    body: formData,
  });

  if (!res.ok) {
    const body: unknown = await res.json().catch(() => null);
    const message =
      body && typeof body === "object" && "error" in body &&
      typeof body.error === "string"
        ? body.error
        : `Request failed with status ${res.status}`;
    throw new ApiError(res.status, message);
  }

  return res.json() as Promise<ImportResponse>;
}
