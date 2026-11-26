import { forwardAccountMutation } from "@/lib/account-api";

export function PATCH(request: Request) {
  return forwardAccountMutation(request, "/api/me");
}
