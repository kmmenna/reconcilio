import { forwardAccountMutation } from "@/lib/account-api";

export function PUT(request: Request) {
  return forwardAccountMutation(request, "/api/me/password");
}
