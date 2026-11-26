import { resolve } from "node:path";

import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US" });

test("previews the Corporate Card CSVs and reloads the same import in both languages", async ({ page }) => {
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  await page.goto(`${origin}/workspace/imports`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("link", { name: "Import CSV" }).click();
  await expect(page.getByRole("heading", { name: "Import CSV" })).toBeVisible();

  for (const [source, fixture, text, reference, side] of [
    ["Demo ERP", "internal.csv", "Cloud hosting subscription", "ERP-001", "Internal"],
    ["Demo Card Statement", "external.csv", "CLOUDHOST* MONTHLY", "CARD-001", "External"],
  ]) {
    await page.getByLabel("Source", { exact: true }).selectOption({ label: source });
    await page.getByLabel("Financial Account").selectOption({ label: "Corporate Card" });
    await page.getByLabel("Start date").fill("2026-11-16");
    await page.getByLabel("End date").fill("2026-11-22");
    await page.getByLabel("CSV file", { exact: true }).setInputFiles(resolve(__dirname, "../../../examples/demo", fixture));
    await page.getByRole("button", { name: "Preview CSV" }).click();
    await expect(page.getByRole("heading", { name: "Import preview" })).toBeVisible();
    await expect(page.getByText("Pending review", { exact: true })).toBeVisible();
    await expect(page.getByText(source, { exact: true })).toBeVisible();
    await expect(page.getByText(side, { exact: true })).toBeVisible();
    const row = page.getByRole("row").filter({ hasText: reference });
    await expect(row.getByText(text, { exact: true })).toBeVisible();
    await expect(row.getByText("-R$89.90", { exact: true })).toBeVisible();

    const previewURL = page.url();
    expect(new URL(previewURL).pathname).toMatch(/^\/workspace\/imports\/[0-9a-f-]{36}$/);
    await page.getByLabel("Language").selectOption("pt-BR");
    await expect(page.getByRole("heading", { name: "Prévia da importação" })).toBeVisible();
    await expect(row.getByText("-R$ 89,90", { exact: true })).toBeVisible();
    await expect(page.getByText("16 de nov. de 2026", { exact: true }).first()).toBeVisible();
    await expect(page).toHaveURL(previewURL);

    await page.reload();
    await expect(page.getByRole("heading", { name: "Prévia da importação" })).toBeVisible();
    await expect(row.getByText(text, { exact: true })).toBeVisible();
    await expect(page).toHaveURL(previewURL);

    await page.getByLabel("Idioma").selectOption("en");
    await page.getByRole("link", { name: "Import another CSV" }).click();
  }

  await page.getByLabel("Source", { exact: true }).selectOption({ label: "Demo ERP" });
  await page.getByLabel("Financial Account").selectOption({ label: "Corporate Card" });
  await page.getByLabel("Start date").fill("2026-11-16");
  await page.getByLabel("End date").fill("2026-11-22");
  await page.getByLabel("CSV file", { exact: true }).setInputFiles({
    name: "invalid.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(
      "source_reference,payment_reference,transaction_date,amount,currency,description\n" +
      "ERP-001,,2026-11-16,-89.901,BRL,Cloud hosting subscription\n",
    ),
  });
  await page.getByRole("button", { name: "Preview CSV" }).click();
  await expect(page.getByRole("alert")).toContainText("Line 2 · Amount");
  await expect(page.getByRole("alert")).toContainText("too many decimal places");
  await expect(page).toHaveURL(`${origin}/workspace/imports`);

  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(page.getByRole("alert")).toContainText("Linha 2 · Valor");
  await expect(page.getByRole("alert")).toContainText("casas decimais demais");
});
