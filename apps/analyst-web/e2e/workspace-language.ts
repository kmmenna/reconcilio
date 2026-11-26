import type { Page } from "@playwright/test";

export async function changeWorkspaceLanguage(page: Page, locale: "en" | "pt-BR") {
  const control = /^(Language|Idioma)$/;
  const option = locale === "en" ? "English" : "Português (Brasil)";
  await page.getByRole("contentinfo").getByRole("button", { name: control }).click();
  await page.getByRole("menu", { name: control }).getByRole("menuitemradio", { name: option, exact: true }).click();
}
