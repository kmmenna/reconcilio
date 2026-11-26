import { readLanguagePreference } from "@/i18n/preference";

import { LanguageSelector } from "./language-selector";

export async function LanguageControl() {
  return <LanguageSelector preference={await readLanguagePreference()} />;
}
