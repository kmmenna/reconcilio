import type catalog from "@/messages/catalog.en.json";
import type en from "@/messages/en.json";
import type imports from "@/messages/imports.en.json";
import type reconciliation from "@/messages/reconciliation.en.json";
import type review from "@/messages/review.en.json";
import type { Locale as SupportedLocale } from "./locales";

declare module "next-intl" {
  interface AppConfig {
    Locale: SupportedLocale;
    Messages: typeof en & { Catalog: typeof catalog; Imports: typeof imports; Reconciliation: typeof reconciliation; Review: typeof review };
  }
}
