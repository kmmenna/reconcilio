export const locales = ["pt-BR", "en"] as const;
export type Locale = (typeof locales)[number];
export type LanguagePreference = Locale | "auto";

export function isLocale(value: unknown): value is Locale {
  return locales.some((locale) => locale === value);
}
