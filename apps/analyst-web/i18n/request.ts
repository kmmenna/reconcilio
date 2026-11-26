import Negotiator from "negotiator";
import { headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";

import catalogEn from "@/messages/catalog.en.json";
import catalogPtBR from "@/messages/catalog.pt-BR.json";
import en from "@/messages/en.json";
import importsEn from "@/messages/imports.en.json";
import importsPtBR from "@/messages/imports.pt-BR.json";
import ptBR from "@/messages/pt-BR.json";
import reconciliationEn from "@/messages/reconciliation.en.json";
import reconciliationPtBR from "@/messages/reconciliation.pt-BR.json";
import reviewEn from "@/messages/review.en.json";
import reviewPtBR from "@/messages/review.pt-BR.json";
import transactionsEn from "@/messages/transactions.en.json";
import transactionsPtBR from "@/messages/transactions.pt-BR.json";

import { type Locale } from "./locales";
import { readLanguagePreference } from "./preference";

const messages = {
  en: { ...en, Catalog: catalogEn, Imports: importsEn, Reconciliation: reconciliationEn, Review: reviewEn, Transactions: transactionsEn },
  "pt-BR": { ...ptBR, Catalog: catalogPtBR, Imports: importsPtBR, Reconciliation: reconciliationPtBR, Review: reviewPtBR, Transactions: transactionsPtBR },
} satisfies Record<Locale, typeof en & {
  Catalog: typeof catalogEn;
  Imports: typeof importsEn;
  Reconciliation: typeof reconciliationEn;
  Review: typeof reviewEn;
  Transactions: typeof transactionsEn;
}>;

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
