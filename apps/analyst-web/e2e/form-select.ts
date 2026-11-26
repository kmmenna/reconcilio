import type { Locator, Page } from "@playwright/test";

export async function selectFormOption(field: Locator, label: string) {
  await field.click();
  await field.page().getByRole("listbox").getByRole("option", { name: label, exact: true }).click();
}

export async function chooseDemoImport(page: Page, source: string) {
  await selectFormOption(page.getByRole("combobox", { name: "Source", exact: true }), source);
  await selectFormOption(page.getByRole("combobox", { name: "Financial Account" }), "Corporate Card");
  const form = page.locator("form").filter({ has: page.getByRole("button", { name: "Preview CSV" }) });
  return form.evaluate((element: HTMLFormElement) => {
    const values = new FormData(element);
    const sourceID = values.get("source_id");
    const accountID = values.get("financial_account_id");
    if (typeof sourceID !== "string" || !sourceID || typeof accountID !== "string" || !accountID) {
      throw new Error("The selected demo Source and Financial Account must be submitted by the form.");
    }
    return { sourceID, accountID };
  });
}
