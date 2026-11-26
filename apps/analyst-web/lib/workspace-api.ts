import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { requestAPI, sessionCookie } from "@/lib/auth";
import { hasTrustedOrigin, problemResponse } from "@/lib/bff";

export async function readWorkspaceResource<T>(path: string, token: string): Promise<T | null | undefined> {
  const response = await requestAPI(path, {
    headers: { Authorization: `Bearer ${token}` },
  }).catch(() => undefined);
  if (response?.status === 401) redirect("/login");
  if (response?.status === 404) return null;
  if (!response?.ok) return undefined;
  return response.json().catch(() => undefined);
}

export async function forwardWorkspaceRequest(
  request: Request,
  path: string,
  unavailable: { code: string; detail: string },
  validate?: (request: Request) => Response | undefined,
) {
  if (request.method !== "GET" && !hasTrustedOrigin(request)) {
    return problemResponse(403, "invalid_origin", "Reload the page and try again.");
  }
  const token = (await cookies()).get(sessionCookie)?.value;
  if (!token) return problemResponse(401, "session_required", "Sign in to continue.");
  const invalid = validate?.(request);
  if (invalid) return invalid;

  try {
    const headers = new Headers({ Authorization: `Bearer ${token}` });
    const requestContentType = request.headers.get("content-type");
    if (requestContentType) headers.set("Content-Type", requestContentType);
    const upstream: RequestInit & { duplex?: "half" } = { method: request.method, headers };
    if (request.body) {
      upstream.body = request.body;
      upstream.duplex = "half";
    }
    const response = await requestAPI(path, upstream);
    const contentType = response.headers.get("content-type");
    if (response.ok || (response.status < 500 && contentType?.startsWith("application/problem+json"))) {
      const headers = new Headers({ "Cache-Control": "no-store" });
      if (contentType) headers.set("Content-Type", contentType);
      return new Response(response.body, { status: response.status, headers });
    }
  } catch {
    // The browser can retry using the same resource URL.
  }
  return problemResponse(503, unavailable.code, unavailable.detail);
}
