import "server-only";

import { headers } from "next/headers";
import { NextResponse } from "next/server";

import { webOrigin } from "@/lib/bff";
import { correlationIDHeader, requestIDHeader } from "@/lib/correlation";

export const sessionCookie = "reconcilio_session";

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

export async function readWorkflowID() {
  return (await headers()).get(correlationIDHeader) ?? crypto.randomUUID();
}

export async function requestAPI(path: string, init: RequestInit = {}) {
  const incoming = await headers();
  const upstream = new Headers(init.headers);
  upstream.set(requestIDHeader, incoming.get(requestIDHeader) ?? crypto.randomUUID());
  upstream.set(correlationIDHeader, incoming.get(correlationIDHeader) ?? crypto.randomUUID());
  const apiOrigin = process.env.BACKEND_API_ORIGIN ?? "http://localhost:8080";
  return fetch(new URL(path, apiOrigin), {
    ...init,
    headers: upstream,
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
