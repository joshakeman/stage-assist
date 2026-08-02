import type { CompareRequest, CompareResponse } from "./types";

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function compareScripts(
  req: CompareRequest,
): Promise<CompareResponse> {
  const res = await fetch("/api/compare", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
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

  return res.json() as Promise<CompareResponse>;
}
