import { cookies } from "next/headers";

import { requestAPI, sessionCookie } from "@/lib/auth";
import { hasTrustedOrigin, problemResponse } from "@/lib/bff";

export async function POST(request: Request) {
  if (!hasTrustedOrigin(request)) {
    return problemResponse(403, "invalid_origin", "Reload the page and try again.");
  }

  const token = (await cookies()).get(sessionCookie)?.value;
  if (!token) return problemResponse(401, "session_required", "Sign in to continue.");

  const contentType = request.headers.get("content-type");
  if (!contentType?.startsWith("multipart/form-data;") || !request.body) {
    return problemResponse(422, "validation_failed", "Select a CSV file.");
  }

  try {
    const upstream: RequestInit & { duplex: "half" } = {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, "Content-Type": contentType },
      body: request.body,
      duplex: "half",
    };
    const response = await requestAPI("/api/imports", upstream);
    return new Response(response.body, {
      status: response.status,
      headers: {
        "Content-Type": response.headers.get("content-type") ?? "application/json",
        "Cache-Control": "no-store",
      },
    });
  } catch {
    return problemResponse(503, "preview_unavailable", "The import preview is temporarily unavailable.");
  }
}
