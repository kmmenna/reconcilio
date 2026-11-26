import { NextResponse } from "next/server";

import { isLocale } from "@/i18n/locales";
import { languageCookie } from "@/i18n/preference";
import { hasTrustedOrigin, problemResponse, webOrigin } from "@/lib/bff";

const preferenceLifetime = 365 * 24 * 60 * 60;

export async function POST(request: Request) {
  if (!hasTrustedOrigin(request)) {
    return problemResponse(403, "invalid_origin", "This request must come from the Reconcilio workspace.");
  }

  const body: unknown = await request.json().catch(() => null);
  const locale = body && typeof body === "object" && "locale" in body ? body.locale : undefined;
  if (locale !== "auto" && !isLocale(locale)) {
    return problemResponse(400, "invalid_language", "Choose one of the available languages.");
  }

  const response = new NextResponse(null, {
    status: 204,
    headers: { "Cache-Control": "no-store" },
  });
  response.cookies.set(languageCookie, locale === "auto" ? "" : locale, {
    path: "/",
    httpOnly: true,
    secure: webOrigin().protocol === "https:",
    sameSite: "lax",
    maxAge: locale === "auto" ? 0 : preferenceLifetime,
  });
  return response;
}
