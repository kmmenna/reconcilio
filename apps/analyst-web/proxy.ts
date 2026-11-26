import { NextResponse, type NextRequest } from "next/server";

import { correlationIDHeader, requestIDHeader, validID } from "@/lib/correlation";

export function proxy(request: NextRequest) {
  const headers = new Headers(request.headers);
  const requestID = validID(headers.get(requestIDHeader)) ?? crypto.randomUUID();
  const correlationID = validID(headers.get(correlationIDHeader))
    ?? validID(request.nextUrl.searchParams.get("workflow"))
    ?? crypto.randomUUID();

  headers.set(requestIDHeader, requestID);
  headers.set(correlationIDHeader, correlationID);
  const response = NextResponse.next({ request: { headers } });
  response.headers.set(requestIDHeader, requestID);
  response.headers.set(correlationIDHeader, correlationID);
  return response;
}

export const config = { matcher: ["/", "/login", "/workspace/:path*", "/api/:path*"] };
