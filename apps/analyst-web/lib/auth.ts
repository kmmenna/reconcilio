import "server-only";

import { NextResponse } from "next/server";

export const sessionCookie = "reconcilio_session";

function webOrigin() {
  return new URL(process.env.WEB_ORIGIN ?? "http://localhost:3000");
}

export function hasTrustedOrigin(request: Request) {
  return request.headers.get("origin") === webOrigin().origin;
}

export function authError(status: number, code: string, detail: string) {
  return NextResponse.json(
    { status, code, detail },
    { status, headers: { "Content-Type": "application/problem+json", "Cache-Control": "no-store" } },
  );
}

export function changeSession(token: string, maxAge: number) {
  const response = new NextResponse(null, {
    status: 204,
    headers: { "Cache-Control": "no-store" },
  });
  response.cookies.set(sessionCookie, token, {
    httpOnly: true,
    secure: webOrigin().protocol === "https:",
    sameSite: "lax",
    path: "/",
    maxAge,
  });
  return response;
}

export function requestAPI(path: string, init: RequestInit = {}) {
  const apiOrigin = process.env.GO_API_ORIGIN ?? "http://localhost:8080";
  return fetch(new URL(path, apiOrigin), {
    ...init,
    cache: "no-store",
    signal: AbortSignal.timeout(5000),
  });
}

type Profile = {
  id: string;
  username: string;
  display_name: string;
  is_admin: boolean;
};

export async function readProfile(token: string): Promise<Profile | null> {
  const response = await requestAPI("/api/me", {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (response.status === 401) return null;
  if (!response.ok) throw new Error("The Go API could not load the profile.");
  return response.json();
}
