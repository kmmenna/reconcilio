import { resolve } from "node:path";

import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US" });

test("confirms both demo CSVs and reads the recorded evidence in either language", async ({ page }) => {
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  await page.goto(`${origin}/workspace/imports`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("link", { name: "Import CSV" }).click();

  for (const [source, fixture, count] of [
    ["Demo ERP", "internal.csv", 3],
    ["Demo Card Statement", "external.csv", 4],
  ] as const) {
    await page.getByLabel("Source", { exact: true }).selectOption({ label: source });
    await page.getByLabel("Financial Account").selectOption({ label: "Corporate Card" });
    await page.getByLabel("Start date").fill("11/16/2026");
    await page.getByLabel("End date").fill("11/22/2026");
    await page.getByLabel("CSV file", { exact: true }).setInputFiles(resolve(__dirname, "../../../examples/demo", fixture));
    await page.getByRole("button", { name: "Preview CSV" }).click();
    await expect(page.getByRole("heading", { name: "Import preview" })).toBeVisible();
    const previewURL = page.url();
    const id = new URL(previewURL).pathname.split("/").at(-1);
    const crossOrigin = await page.request.post(`${origin}/api/imports/${id}/confirm`, {
      headers: { Origin: "https://example.invalid" },
    });
    expect(crossOrigin.status()).toBe(403);

    await page.getByRole("button", { name: "Confirm import" }).click();
    await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();
    await expect(page.getByText(`${count} transactions imported`, { exact: true })).toBeVisible();
    await expect(page).toHaveURL(previewURL);
    await page.reload();
    await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();

    await page.getByLabel("Language").selectOption("pt-BR");
    await expect(page.getByRole("heading", { name: "Importação confirmada" })).toBeVisible();
    await page.getByRole("link", { name: "Acompanhar processamento" }).click();
    await expect(page.getByRole("heading", { name: "Processamento da conciliação" })).toBeVisible();
    await expect(page.getByRole("status")).toHaveText("Concluído", { timeout: 15_000 });
    const jobURL = page.url();

    await page.getByLabel("Idioma").selectOption("en");
    await expect(page.getByRole("status")).toHaveText("Completed");
    await expect(page).toHaveURL(jobURL);

    if (fixture === "internal.csv") {
      await page.goto(`${origin}/workspace/imports`);
    } else {
      await page.getByRole("link", { name: "View Reconciliations" }).click();
    }
  }

  await expect(page.getByRole("heading", { name: "Reconciliations" })).toBeVisible();
  await expect(page.getByLabel("Financial Account")).toHaveValue(/^[0-9a-f-]{36}$/);
  await page.getByLabel("Financial Account").selectOption({ label: "All Financial Accounts" });
  await page.getByRole("button", { name: "Apply filter" }).click();
  await page.waitForURL((url) => url.pathname === "/workspace/reconciliations" && !url.searchParams.get("financial_account_id"));
  await expect(page.getByRole("link", { name: "View details" })).toHaveCount(1);
  await expect(page.getByText("-R$89.90", { exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Automatic", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "View details" }).click();

  await expect(page.getByRole("heading", { name: "Reconciliation detail" })).toBeVisible();
  const detailURL = page.url();
  expect(new URL(detailURL).pathname).toMatch(/^\/workspace\/reconciliations\/[0-9a-f-]{36}$/);
  await expect(page.getByText("Cloud hosting subscription", { exact: true })).toBeVisible();
  await expect(page.getByText("CLOUDHOST* MONTHLY", { exact: true })).toBeVisible();
  await expect(page.getByText("ERP-001", { exact: true })).toBeVisible();
  await expect(page.getByText("CARD-001", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Recorded evidence" })).toBeVisible();

  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(page.getByRole("heading", { name: "Detalhes da conciliação" })).toBeVisible();
  await expect(page.getByText("-R$ 89,90", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("16 de nov. de 2026", { exact: true }).first()).toBeVisible();
  await page.reload();
  await expect(page).toHaveURL(detailURL);
  await expect(page.getByRole("heading", { name: "Evidências registradas" })).toBeVisible();
  await expect(page.getByText("CLOUDHOST* MONTHLY", { exact: true })).toBeVisible();
});
