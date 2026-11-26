import "server-only";

import { NextResponse } from "next/server";

export function webOrigin() {
  return new URL(process.env.WEB_ORIGIN ?? "http://localhost:3000");
}

export function hasTrustedOrigin(request: Request) {
  return request.headers.get("origin") === webOrigin().origin;
}

export function problemResponse(status: number, code: string, detail: string) {
  return NextResponse.json(
    { status, code, detail },
    { status, headers: { "Content-Type": "application/problem+json", "Cache-Control": "no-store" } },
  );
}
