import { authError, changeSession, hasTrustedOrigin } from "@/lib/auth";

export function POST(request: Request) {
  if (!hasTrustedOrigin(request)) {
    return authError(403, "invalid_origin", "This request must come from the Reconcilio workspace.");
  }
  return changeSession("", 0);
}
