import { resolve } from "node:path";

import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US" });

test("confirms demo imports, keeps or cancels reimport conflicts, and reads receipts in either language", async ({ page }) => {
  test.setTimeout(60_000);
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

  async function uploadReimport(fixture: string | { name: string; mimeType: string; buffer: Buffer }) {
    await page.goto(`${origin}/workspace/imports`);
    await page.getByLabel("Source", { exact: true }).selectOption({ label: "Demo ERP" });
    await page.getByLabel("Financial Account").selectOption({ label: "Corporate Card" });
    await page.getByLabel("Start date").fill("11/16/2026");
    await page.getByLabel("End date").fill("11/22/2026");
    await page.getByLabel("CSV file", { exact: true }).setInputFiles(fixture);
    await page.getByRole("button", { name: "Preview CSV" }).click();
  }

  await page.getByLabel("Idioma").selectOption("en");
  await expect(page.getByRole("heading", { name: "Reconciliation detail" })).toBeVisible();
  const conflictFixture = resolve(__dirname, "../../../examples/test-cases/reimport-amount-conflict.csv");
  await uploadReimport(conflictFixture);
  const amountComparison = page.getByRole("table", { name: /ERP-001/ });
  await expect(amountComparison.getByRole("columnheader", { name: "Stored Transaction" })).toBeVisible();
  await expect(amountComparison.getByRole("cell", { name: "-R$89.90", exact: true })).toBeVisible();
  await expect(amountComparison.getByRole("cell", { name: "-R$94.90", exact: true })).toBeVisible();
  const conflictURL = page.url();
  const cancelPath = conflictURL.replace("/workspace/", "/api/") + "/cancel";
  const crossOriginCancel = await page.request.post(cancelPath, { headers: { Origin: "https://example.invalid" } });
  expect(crossOriginCancel.status()).toBe(403);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(page.getByRole("heading", { name: "Conflitos de referência da fonte" })).toBeVisible();
  await expect(amountComparison.getByRole("columnheader", { name: "Transação registrada" })).toBeVisible();
  await expect(amountComparison.getByRole("cell", { name: "-R$ 89,90", exact: true })).toBeVisible();
  await expect(amountComparison.getByRole("cell", { name: "-R$ 94,90", exact: true })).toBeVisible();
  await expect(page).toHaveURL(conflictURL);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
  await page.getByRole("button", { name: "Cancelar importação" }).click();
  await expect(page.getByRole("heading", { name: "Importação cancelada" })).toBeVisible();
  await expect(page.getByText("Nenhuma transação foi importada.")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Decisão registrada" })).toBeVisible();
  await page.reload();
  await expect(page).toHaveURL(conflictURL);
  await expect(amountComparison.getByRole("cell", { name: "-R$ 89,90", exact: true })).toBeVisible();

  await page.getByLabel("Idioma").selectOption("en");
  await expect(page.getByRole("heading", { name: "Import canceled" })).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 900 });
  await uploadReimport(conflictFixture);
  await page.getByRole("button", { name: "Keep originals and import" }).click();
  await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();
  await expect(page.getByText("1 transaction imported", { exact: true })).toBeVisible();
  await expect(page.getByText("The original Transactions were kept.")).toBeVisible();
  const receiptURL = page.url();
  await uploadReimport(conflictFixture);
  await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();
  await expect(page).toHaveURL(receiptURL);

  await page.goto(detailURL);
  await expect(page.getByRole("heading", { name: "Reconciliation detail" })).toBeVisible();
  await expect(page.getByText("-R$89.90", { exact: true }).first()).toBeVisible();

  const header = "source_reference,payment_reference,transaction_date,amount,currency,description\n";
  await uploadReimport({
    name: "newer.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(header + "ERP-WEB-RACE,,2026-11-20,21.00,BRL,Incoming from browser\n"),
  });
  await expect(page.getByRole("heading", { name: "Import preview" })).toBeVisible();
  const stagedURL = page.url();
  await page.goto(`${origin}/workspace/imports`);
  const sourceID = await page.getByLabel("Source", { exact: true }).locator("option").filter({ hasText: "Demo ERP" }).getAttribute("value");
  const accountID = await page.getByLabel("Financial Account").locator("option").filter({ hasText: "Corporate Card" }).getAttribute("value");
  expect(sourceID).toBeTruthy();
  expect(accountID).toBeTruthy();
  const competing = await page.request.post(`${origin}/api/imports`, {
    headers: { Origin: origin },
    multipart: {
      source_id: sourceID!,
      financial_account_id: accountID!,
      period_start: "2026-11-16",
      period_end: "2026-11-22",
      file: { name: "earlier.csv", mimeType: "text/csv", buffer: Buffer.from(header + "ERP-WEB-RACE,,2026-11-20,20.00,BRL,Competing import\n") },
    },
  });
  expect(competing.status()).toBe(201);
  const { id: competingID } = await competing.json();
  const concurrentConfirmation = await page.request.post(`${origin}/api/imports/${competingID}/confirm`, { headers: { Origin: origin } });
  expect(concurrentConfirmation.status()).toBe(200);
  // Open the original staged URL, then let the server find the conflict at confirmation.
  await page.goto(stagedURL);
  await page.getByRole("button", { name: "Confirm import" }).click();
  const changed = page.getByRole("alert").filter({ hasText: "The conflicts changed since the preview." });
  await expect(changed).toBeFocused();
  await expect(page).toHaveURL(stagedURL);
  await expect(page.getByRole("table", { name: /ERP-WEB-RACE/ }).getByRole("cell", { name: "R$20.00" })).toBeVisible();
  await page.getByRole("button", { name: "Keep originals and import" }).click();
  await expect(page.getByText("0 transactions imported", { exact: true })).toBeVisible();

  await uploadReimport({
    name: "reference-free.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(header + ",,2026-11-22,1.00,BRL,No Source Reference\n"),
  });
  const overlap = page.getByRole("alert").filter({ hasText: "The dates overlap an earlier import" });
  await expect(overlap).toContainText("Corporate Card covers Nov 16, 2026 through Nov 22, 2026");
  await page.getByLabel("Language").selectOption("pt-BR");
  const localizedOverlap = page.getByRole("alert").filter({ hasText: "As datas se sobrepõem" });
  await expect(localizedOverlap).toContainText("Corporate Card abrange o período de 16 de nov. de 2026 a 22 de nov. de 2026");

});
