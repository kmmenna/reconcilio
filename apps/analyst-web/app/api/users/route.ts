import { forwardAccountMutation } from "@/lib/account-api";

export function POST(request: Request) {
  return forwardAccountMutation(request, "/api/users");
}
