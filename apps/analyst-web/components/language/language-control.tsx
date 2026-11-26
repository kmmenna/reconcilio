import { readLanguagePreference } from "@/i18n/preference";

import { LanguageSelector } from "./language-selector";

export async function LanguageControl({ side = "bottom" }: { side?: "top" | "bottom" }) {
  return <LanguageSelector preference={await readLanguagePreference()} side={side} />;
}
