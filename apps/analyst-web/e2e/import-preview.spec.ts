import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

import { expect, test } from "@playwright/test";

import { changeWorkspaceLanguage } from "./workspace-language";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US", timezoneId: "America/Los_Angeles" });

test("previews the Corporate Card CSVs and reloads the same import in both languages", async ({ page }) => {
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  const unauthenticated = await page.request.post(`${origin}/api/imports`, {
    headers: { Origin: origin },
  });
  expect(unauthenticated.status()).toBe(401);
  expect(await unauthenticated.json()).toMatchObject({ code: "session_required" });

  await page.goto(`${origin}/workspace/imports`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Import CSV" })).toBeVisible();
  const unsupportedMedia = await page.request.post(`${origin}/api/imports`, {
    headers: { Origin: origin },
    data: { file: "invalid" },
  });
  expect(unsupportedMedia.status()).toBe(422);
  expect(await unsupportedMedia.json()).toMatchObject({ code: "validation_failed" });
  const crossOrigin = await page.request.post(`${origin}/api/imports`, {
    headers: { Origin: "https://example.invalid" },
  });
  expect(crossOrigin.status()).toBe(403);

  const startDate = page.getByLabel("Start date", { exact: true });
  const startCalendar = page.getByRole("button", { name: "Open calendar for Start date" });
  await startDate.fill("11/15/2026");
  await startCalendar.press("Enter");
  const calendar = page.getByRole("dialog", { name: "Calendar for Start date" });
  await expect(calendar.getByRole("button", { name: /Sunday, November 15, 2026$/ })).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await expect(calendar.getByRole("button", { name: /Monday, November 16, 2026$/ })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(calendar).toBeHidden();
  await expect(startCalendar).toBeFocused();
  await expect(startDate).toHaveValue("11/16/2026");

  for (const [source, fixture, text, reference, side] of [
    ["Demo ERP", "internal.csv", "Cloud hosting subscription", "ERP-001", "Internal"],
    ["Demo Card Statement", "external.csv", "CLOUDHOST* MONTHLY", "CARD-001", "External"],
  ]) {
    await page.getByLabel("Source", { exact: true }).selectOption({ label: source });
    await page.getByLabel("Financial Account").selectOption({ label: "Corporate Card" });
    if (source !== "Demo ERP") await startDate.fill("11/16/2026");
    await page.getByLabel("End date", { exact: true }).fill("11/22/2026");
    const data = await readFile(resolve(__dirname, "../../../examples/demo", fixture));
    // Keep this preview distinct from the confirmation smoke test's exact file replay.
    await page.getByLabel("CSV file", { exact: true }).setInputFiles({
      name: fixture, mimeType: "text/csv", buffer: Buffer.concat([data, Buffer.from("\n")]),
    });
    await page.getByRole("button", { name: "Preview CSV" }).click();
    await expect(page.getByRole("heading", { name: "Import preview" })).toBeVisible();
    await expect(page.getByText("Pending review", { exact: true })).toBeVisible();
    await expect(page.getByText(source, { exact: true })).toBeVisible();
    await expect(page.getByText("Corporate Card", { exact: true })).toBeVisible();
    await expect(page.getByText(side, { exact: true })).toBeVisible();
    const row = page.getByRole("row").filter({ hasText: reference });
    await expect(row.getByText(text, { exact: true })).toBeVisible();
    await expect(row.getByText("-R$89.90", { exact: true })).toBeVisible();

    const previewURL = page.url();
    expect(new URL(previewURL).pathname).toMatch(/^\/workspace\/imports\/[0-9a-f-]{36}$/);
    await changeWorkspaceLanguage(page, "pt-BR");
    await expect(page.getByRole("heading", { name: "Prévia da importação" })).toBeVisible();
    await expect(row.getByText("-R$ 89,90", { exact: true })).toBeVisible();
    await expect(page.locator('time[datetime="2026-11-16"]').first()).toHaveText("16 de nov. de 2026");
    await expect(page).toHaveURL(previewURL);

    await page.reload();
    await expect(page.getByRole("heading", { name: "Prévia da importação" })).toBeVisible();
    await expect(row.getByText(text, { exact: true })).toBeVisible();
    await expect(page).toHaveURL(previewURL);

    await changeWorkspaceLanguage(page, "en");
    await page.getByRole("link", { name: "Import another CSV" }).click();
  }

  await page.getByLabel("Source", { exact: true }).selectOption({ label: "Demo ERP" });
  await page.getByLabel("Financial Account").selectOption({ label: "Corporate Card" });
  await page.getByLabel("Start date", { exact: true }).fill("11/16/2026");
  await page.getByLabel("End date", { exact: true }).fill("11/22/2026");
  await page.getByLabel("CSV file", { exact: true }).setInputFiles({
    name: "invalid.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(
      "source_reference,payment_reference,transaction_date,amount,currency,description\n" +
      "ERP-001,,2026-11-16,-89.901,BRL,Cloud hosting subscription\n",
    ),
  });
  await page.getByRole("button", { name: "Preview CSV" }).click();
  const error = page.getByRole("main").getByRole("alert");
  await expect(error).toContainText("Line 2 · Amount");
  await expect(error).toContainText("too many decimal places");
  await expect(page).toHaveURL(`${origin}/workspace/imports`);

  await changeWorkspaceLanguage(page, "pt-BR");
  await expect(error).toContainText("Linha 2 · Valor");
  await expect(error).toContainText("casas decimais demais");
  await expect(page.getByLabel("Data inicial", { exact: true })).toHaveValue("16/11/2026");
  await expect(page.getByLabel("Data final", { exact: true })).toHaveValue("22/11/2026");
  const endCalendar = page.getByRole("button", { name: "Abrir calendário — Data final" });
  await page.setViewportSize({ width: 320, height: 700 });
  await endCalendar.click();
  const localizedCalendar = page.getByRole("dialog", { name: "Calendário — Data final" });
  await expect(localizedCalendar.getByRole("button", { name: "Mês anterior" })).toBeVisible();
  await localizedCalendar.getByRole("button", { name: /segunda-feira, 23 de novembro de 2026$/ }).click();
  await expect(localizedCalendar).toBeHidden();
  await expect(page.getByLabel("Data final", { exact: true })).toHaveValue("23/11/2026");
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);
});
