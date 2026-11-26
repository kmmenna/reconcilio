import { expect, type Page } from "@playwright/test";

export async function verifyOpenTransactions(page: Page) {
  await page.getByRole("link", { name: "Reconcilio" }).click();
  await page.getByRole("link", { name: "Open Transactions" }).click();
  await expect(page.getByRole("heading", { name: "Open Transactions" })).toBeVisible();
  const openURL = page.url();
  await expect(page.getByRole("table").locator("tbody").getByRole("row")).toHaveCount(5);
  const row = (reference: string) => page.getByRole("row").filter({ hasText: reference });
  const internal = row("ERP-003");
  const external = row("CARD-004");

  await expect(row("ERP-001")).toHaveCount(0);
  await expect(row("CARD-001")).toHaveCount(0);
  for (const [entry, side, description, amount, date] of [
    [internal, "Internal", "Office supplies order", "-R$120.00", "Nov 20, 2026"],
    [external, "External", "OFFICE SUPPLIES STORE", "-R$119.00", "Nov 21, 2026"],
  ] as const) {
    await expect(entry.getByText(side, { exact: true })).toBeVisible();
    await expect(entry.getByText(description, { exact: true })).toBeVisible();
    await expect(entry.getByText(amount, { exact: true })).toBeVisible();
    await expect(entry.getByText(date, { exact: true })).toBeVisible();
    await expect(entry).toContainText("DEMO-PAY-003");
    await expect(entry).toContainText("Amount difference");
    await expect(entry.getByText("R$1.00", { exact: true })).toBeVisible();
    await expect(entry.getByText("No pending proposals", { exact: true })).toBeVisible();
  }

  for (const reference of ["ERP-002", "CARD-002", "CARD-003"]) {
    await expect(row(reference).getByText("Pending proposals", { exact: true })).toBeVisible();
  }

  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(page.getByRole("heading", { name: "Transações em aberto" })).toBeVisible();
  await expect(page).toHaveURL(openURL);
  await page.reload();
  await expect(internal.getByText("Interna", { exact: true })).toBeVisible();
  await expect(external.getByText("Externa", { exact: true })).toBeVisible();
  await expect(internal.getByText("R$ 1,00", { exact: true })).toBeVisible();
  await expect(internal.getByText("-R$ 120,00", { exact: true })).toBeVisible();
  await expect(internal.getByText("20 de nov. de 2026", { exact: true })).toBeVisible();
  await expect(internal.getByText("Office supplies order", { exact: true })).toBeVisible();
  await expect(external.getByText("OFFICE SUPPLIES STORE", { exact: true })).toBeVisible();
  await expect(internal).toContainText("Diferença de valor");
  await expect(row("ERP-002").getByText("Propostas pendentes", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

  await page.getByLabel("Idioma").selectOption("en");
  await row("ERP-002").getByRole("link", { name: "Pending proposals" }).click();
  await expect(page.getByRole("heading", { name: "Compare Match Proposals" })).toBeVisible();
  await expect(page.getByRole("article", { name: "CANVA* I045" })).toBeVisible();
  await expect(page.getByRole("article", { name: "ADOBE* CLOUD" })).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 900 });
}
