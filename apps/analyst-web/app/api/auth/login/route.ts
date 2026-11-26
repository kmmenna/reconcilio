import { authError, changeSession, hasTrustedOrigin, requestAPI } from "@/lib/auth";

export async function POST(request: Request) {
  if (!hasTrustedOrigin(request)) {
    return authError(403, "invalid_origin", "This request must come from the Reconcilio workspace.");
  }
  if (request.headers.get("content-type")?.split(";")[0] !== "application/json") {
    return authError(415, "invalid_content_type", "Send the sign-in credentials as JSON.");
  }

  try {
    const response = await requestAPI("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: await request.text(),
    });
    if (response.status === 401) {
      return authError(401, "invalid_credentials", "The username or password is incorrect.");
    }
    if ([400, 413, 422].includes(response.status)) {
      return authError(response.status, "invalid_sign_in", "Enter a valid username and password.");
    }
    if (response.ok) {
      const { access_token } = await response.json();
      if (typeof access_token === "string" && access_token) {
        return changeSession(access_token, 30 * 60);
      }
    }
  } catch {
    // The browser receives the same recoverable response for upstream failures.
  }
  return authError(503, "sign_in_unavailable", "Sign-in is temporarily unavailable. Try again in a moment.");
}
