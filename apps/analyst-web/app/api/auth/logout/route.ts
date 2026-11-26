import { changeSession } from "@/lib/auth";
import { hasTrustedOrigin, problemResponse } from "@/lib/bff";

export function POST(request: Request) {
  if (!hasTrustedOrigin(request)) {
    return problemResponse(403, "invalid_origin", "This request must come from the Reconcilio workspace.");
  }
  return changeSession("", 0);
}
