import Negotiator from "negotiator";
import { headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";

import en from "@/messages/en.json";
import importsEn from "@/messages/imports.en.json";
import importsPtBR from "@/messages/imports.pt-BR.json";
import ptBR from "@/messages/pt-BR.json";

import { type Locale } from "./locales";
import { readLanguagePreference } from "./preference";

const messages = {
  en: { ...en, Imports: importsEn },
  "pt-BR": { ...ptBR, Imports: importsPtBR },
} satisfies Record<Locale, typeof en & { Imports: typeof importsEn }>;

async function detectBrowserLocale(): Promise<Locale> {
  const acceptLanguage = (await headers()).get("accept-language") ?? "";
  const preferred = new Negotiator({ headers: { "accept-language": acceptLanguage } }).languages();

  for (const tag of preferred) {
    const language = tag.split("-", 1)[0].toLowerCase();
    if (language === "pt") return "pt-BR";
    if (language === "en") return "en";
  }
  return "en";
}

export default getRequestConfig(async () => {
  const preference = await readLanguagePreference();
  const locale = preference === "auto" ? await detectBrowserLocale() : preference;
  return { locale, messages: messages[locale] };
});
