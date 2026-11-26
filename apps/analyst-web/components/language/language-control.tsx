import { readLanguagePreference } from "@/i18n/preference";

import { LanguageSelector, type LanguagePresentation } from "./language-selector";

export async function LanguageControl({ presentation = "select" }: { presentation?: LanguagePresentation }) {
  return <LanguageSelector preference={await readLanguagePreference()} presentation={presentation} />;
}
