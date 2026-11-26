import "server-only";

import { cookies } from "next/headers";

import { requestAPI, sessionCookie } from "@/lib/auth";
import { hasTrustedOrigin, problemResponse } from "@/lib/bff";

export async function forwardAccountMutation(request: Request, path: string) {
  if (!hasTrustedOrigin(request)) {
    return problemResponse(403, "invalid_origin", "This request must come from the Reconcilio workspace.");
  }
  const token = (await cookies()).get(sessionCookie)?.value;
  if (!token) {
    return problemResponse(401, "session_required", "Sign in to continue.");
  }
  if (request.headers.get("content-type")?.split(";")[0].trim().toLowerCase() !== "application/json") {
    return problemResponse(415, "invalid_content_type", "Send the account fields as JSON.");
  }

  try {
    const response = await requestAPI(path, {
      method: request.method,
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
      body: await request.text(),
    });
    const contentType = response.headers.get("content-type");
    if (response.ok || (response.status < 500 && contentType?.startsWith("application/problem+json"))) {
      const headers = new Headers({ "Cache-Control": "no-store" });
      if (contentType) headers.set("Content-Type", contentType);
      return new Response(response.body, { status: response.status, headers });
    }
  } catch {
    // Upstream failures share a recoverable response.
  }
  return problemResponse(503, "account_unavailable", "The account service is temporarily unavailable.");
}
