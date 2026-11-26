import type en from "@/messages/en.json";
import type { Locale as SupportedLocale } from "./locales";

declare module "next-intl" {
  interface AppConfig {
    Locale: SupportedLocale;
    Messages: typeof en;
  }
}
