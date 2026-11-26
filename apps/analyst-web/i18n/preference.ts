import "server-only";

import { cookies } from "next/headers";

import { isLocale, type LanguagePreference } from "./locales";

export const languageCookie = "reconcilio_locale";

export async function readLanguagePreference(): Promise<LanguagePreference> {
  const locale = (await cookies()).get(languageCookie)?.value;
  return isLocale(locale) ? locale : "auto";
}
