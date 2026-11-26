import { resolve } from "node:path";

import { expect, test } from "@playwright/test";

import { verifyOpenTransactions } from "./open-transactions";
import { changeWorkspaceLanguage } from "./workspace-language";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US" });

test("reconciles demo imports, reviews reimport conflicts, and preserves proposal history in both languages", async ({ page, context }) => {
  test.setTimeout(60_000);
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  await page.goto(`${origin}/workspace/imports`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  const navigation = page.getByRole("navigation", { name: /^(Workspace navigation|Navegação da área de conciliação)$/ });
  const currentSection = navigation.locator('[aria-current="location"]');

  for (const [source, fixture, count] of [
    ["Demo ERP", "internal.csv", 3],
    ["Demo Card Statement", "external.csv", 4],
  ] as const) {
    await page.getByLabel("Source", { exact: true }).selectOption({ label: source });
    await page.getByLabel("Financial Account").selectOption({ label: "Corporate Card" });
    await page.getByLabel("Start date").fill("11/16/2026");
    await page.getByLabel("End date").fill("11/22/2026");
    await page.getByLabel("CSV file", { exact: true }).setInputFiles(resolve(__dirname, "../../../examples/demo", fixture));
    const previewRequest = page.waitForResponse(response => response.url() === `${origin}/api/imports` && response.request().method() === "POST");
    await page.getByRole("button", { name: "Preview CSV" }).click();
    const staged = await previewRequest;
    const stagedHeaders = await staged.request().allHeaders();
    const workflow = stagedHeaders["x-correlation-id"];
    expect(workflow).toMatch(/^[0-9a-f-]{36}$/);
    expect(stagedHeaders["x-request-id"]).toMatch(/^[0-9a-f-]{36}$/);
    expect(staged.headers()["x-request-id"]).toBe(stagedHeaders["x-request-id"]);
    await expect(page.getByRole("heading", { name: "Import preview" })).toBeVisible();
    const previewURL = page.url();
    const id = new URL(previewURL).pathname.split("/").at(-1);
    const crossOrigin = await page.request.post(`${origin}/api/imports/${id}/confirm`, {
      headers: { Origin: "https://example.invalid" },
    });
    expect(crossOrigin.status()).toBe(403);

    const confirmRequest = page.waitForResponse(response => response.url() === `${origin}/api/imports/${id}/confirm` && response.ok());
    await page.getByRole("button", { name: "Confirm import" }).click();
    const confirmation = await confirmRequest;
    const confirmHeaders = await confirmation.request().allHeaders();
    expect(confirmHeaders["x-correlation-id"]).toBe(workflow);
    expect(confirmation.headers()["x-correlation-id"]).toBe(workflow);
    expect(confirmation.headers()["x-request-id"]).toBe(confirmHeaders["x-request-id"]);
    expect(confirmHeaders["x-request-id"]).not.toBe(stagedHeaders["x-request-id"]);
    await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();
    await expect(page.getByText(`${count} transactions imported`, { exact: true })).toBeVisible();
    await expect(page).toHaveURL(previewURL);
    await page.reload();
    await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();
    await expect(currentSection).toHaveText("Import CSV");

    await changeWorkspaceLanguage(page, "pt-BR");
    await expect(page.getByRole("heading", { name: "Importação confirmada" })).toBeVisible();
    await page.getByRole("link", { name: "Acompanhar processamento" }).click();
    await expect(page.getByRole("heading", { name: "Processamento da conciliação" })).toBeVisible();
    await expect(page.getByRole("status")).toHaveText("Concluído", { timeout: 15_000 });
    const jobURL = page.url();

    await changeWorkspaceLanguage(page, "en");
    await expect(page.getByRole("status")).toHaveText("Completed");
    await expect(page).toHaveURL(jobURL);

    if (fixture === "internal.csv") {
      await expect(page.getByText("No automatic Reconciliations", { exact: true })).toBeVisible();
      await expect(page.getByText("No new Match Proposals", { exact: true })).toBeVisible();
      await navigation.getByRole("link", { name: "Import CSV", exact: true }).click();
    } else {
      await expect(page.getByText("1 automatic Reconciliation", { exact: true })).toBeVisible();
      await expect(page.getByText("2 new Match Proposals", { exact: true })).toBeVisible();
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

  await verifyOpenTransactions(page);
  await page.goto(detailURL);
  await expect(page).toHaveURL(detailURL);
  await expect(currentSection).toHaveText("Reconciliations");

  await changeWorkspaceLanguage(page, "pt-BR");
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

  await changeWorkspaceLanguage(page, "en");
  await expect(page.getByRole("heading", { name: "Reconciliation detail" })).toBeVisible();
  const conflictFixture = resolve(__dirname, "../../../examples/test-cases/reimport-amount-conflict.csv");
  await uploadReimport(conflictFixture);
  const amountComparison = page.getByRole("table", { name: /ERP-001/ });
  await expect(amountComparison.getByRole("columnheader", { name: "Stored Transaction" })).toBeVisible();
  await expect(amountComparison.getByRole("cell", { name: "-R$89.90", exact: true })).toBeVisible();
  await expect(amountComparison.getByRole("cell", { name: "-R$94.90", exact: true })).toBeVisible();
  const conflictURL = page.url();
  const cancelPath = `${origin}${new URL(conflictURL).pathname.replace("/workspace/", "/api/")}/cancel`;
  const crossOriginCancel = await page.request.post(cancelPath, { headers: { Origin: "https://example.invalid" } });
  expect(crossOriginCancel.status()).toBe(403);

  await page.setViewportSize({ width: 390, height: 844 });
  await changeWorkspaceLanguage(page, "pt-BR");
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

  await changeWorkspaceLanguage(page, "en");
  await expect(page.getByRole("heading", { name: "Import canceled" })).toBeVisible();
  await expect(page.getByText("The import was canceled with these conflicts.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Track processing" })).toHaveCount(0);
  await page.goto(`${origin}/workspace/transactions`);
  await expect(page.getByText("ERP-003", { exact: true })).toBeVisible();
  await expect(page.getByText("ERP-004", { exact: true })).toHaveCount(0);
  await page.goto(detailURL);
  await expect(page.getByText("-R$89.90", { exact: true }).first()).toBeVisible();

  await page.setViewportSize({ width: 1280, height: 900 });
  await uploadReimport(conflictFixture);
  await page.getByRole("button", { name: "Keep originals and import" }).click();
  await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();
  await expect(page.getByText("1 transaction imported", { exact: true })).toBeVisible();
  await expect(page.getByText("The original Transactions were kept.")).toBeVisible();
  const receiptURL = page.url();
  await uploadReimport(conflictFixture);
  await expect(page.getByRole("heading", { name: "Import confirmed" })).toBeVisible();
  await expect(page).toHaveURL(url => url.pathname === new URL(receiptURL).pathname);
  await expect(page.getByRole("heading", { name: "Recorded decision" })).toBeVisible();
  await expect(page.getByText("The original Transactions were kept.")).toBeVisible();

  await page.goto(`${origin}/workspace/transactions`);
  const imported = page.getByRole("row").filter({ hasText: "ERP-004" });
  await expect(imported.getByText("Office parking", { exact: true })).toBeVisible();
  await expect(imported.getByText("-R$34.50", { exact: true })).toBeVisible();

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
  await changeWorkspaceLanguage(page, "pt-BR");
  const localizedOverlap = page.getByRole("alert").filter({ hasText: "As datas se sobrepõem" });
  await expect(localizedOverlap).toContainText("Corporate Card abrange o período de 16 de nov. de 2026 a 22 de nov. de 2026");

  await page.goto(detailURL);
  await changeWorkspaceLanguage(page, "en");
  await expect(page.getByRole("heading", { name: "Reconciliation detail" })).toBeVisible();
  await navigation.getByRole("link", { name: "Review queue", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Review queue" })).toBeVisible();
  await expect(page.getByText("ERP-002", { exact: true })).toBeVisible();
  await expect(page.getByText("2 pending alternatives", { exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Compare alternatives" }).click();

  await expect(page.getByRole("heading", { name: "Compare Match Proposals" })).toBeVisible();
  await expect(currentSection).toHaveText("Review queue");
  const groupURL = page.url();
  const canva = page.getByRole("article", { name: "CANVA* I045" });
  const adobe = page.getByRole("article", { name: "ADOBE* CLOUD" });
  await expect(canva.getByText("CARD-002", { exact: true })).toBeVisible();
  await expect(adobe.getByText("CARD-003", { exact: true })).toBeVisible();
  for (const alternative of [canva, adobe]) {
    await expect(alternative.getByText("Equal signed amount", { exact: true })).toBeVisible();
    await expect(alternative.getByText("1 day", { exact: true })).toBeVisible();
  }
  const adobeLink = await adobe.getByRole("link", { name: "View proposal" }).getAttribute("href");
  if (!adobeLink) throw new Error("The Adobe alternative is missing its direct proposal link.");

  await canva.getByRole("link", { name: "View proposal" }).click();
  await expect(page.getByText("Pending", { exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Back to alternatives" }).click();
  await expect(page).toHaveURL(groupURL);

  const staleProposal = await context.newPage();
  await staleProposal.goto(new URL(adobeLink, origin).href);
  await expect(staleProposal.getByText("Pending", { exact: true })).toBeVisible();
  await expect(staleProposal.getByRole("navigation", { name: "Workspace navigation" }).locator('[aria-current="location"]'))
    .toHaveText("Review queue");
  await changeWorkspaceLanguage(staleProposal, "pt-BR");
  await expect(staleProposal.getByRole("heading", { name: "Evidências originais da proposta" })).toBeVisible();
  await expect(staleProposal).toHaveURL(new URL(adobeLink, origin).href);
  const crossOriginDecision = await page.request.post(`${origin}/api${adobeLink.replace("/workspace", "")}/reject`, {
    headers: { Origin: "https://example.invalid" },
  });
  expect(crossOriginDecision.status()).toBe(403);

  const canvaPath = await canva.getByRole("link", { name: "View proposal" }).getAttribute("href");
  if (!canvaPath) throw new Error("The Canva alternative is missing its direct proposal link.");
  const [decision, detail] = await Promise.all([
    page.waitForResponse(response => new URL(response.url()).pathname === `/api${canvaPath.replace("/workspace", "")}/confirm`),
    page.waitForResponse(response => new URL(response.url()).pathname === canvaPath),
    canva.getByRole("button", { name: "Confirm match" }).click(),
  ]);
  await expect(page.getByText(/^(Accepted|Aceita)$/)).toBeVisible();
  const reviewWorkflow = decision.headers()["x-correlation-id"];
  expect(reviewWorkflow).toMatch(/^[0-9a-f-]{36}$/);
  expect(detail.headers()["x-correlation-id"]).toBe(reviewWorkflow);
  expect(detail.headers()["x-request-id"]).not.toBe(decision.headers()["x-request-id"]);
  expect(new URL(page.url()).searchParams.get("workflow")).toBe(reviewWorkflow);
  const reread = await page.reload();
  expect(reread?.headers()["x-correlation-id"]).toBe(reviewWorkflow);
  expect(reread?.headers()["x-request-id"]).not.toBe(detail.headers()["x-request-id"]);
  await expect(page.getByText(/^(Accepted|Aceita)$/)).toBeVisible();
  await page.goto(groupURL);
  await expect(page.getByText("Não há alternativas pendentes para esta transação.", { exact: true })).toBeVisible();
  await navigation.getByRole("link", { name: "Fila de revisão", exact: true }).click();
  await expect(page.getByText("Nenhuma proposta de conciliação aguarda revisão.", { exact: true })).toBeVisible();

  const [conflict, refreshed] = await Promise.all([
    staleProposal.waitForResponse(response => new URL(response.url()).pathname === `/api${adobeLink.replace("/workspace", "")}/confirm`),
    staleProposal.waitForResponse(response => new URL(response.url()).pathname === adobeLink),
    staleProposal.getByRole("button", { name: "Confirmar conciliação" }).click(),
  ]);
  expect(conflict.status()).toBe(409);
  expect(refreshed.headers()["x-correlation-id"]).toBe(conflict.headers()["x-correlation-id"]);
  expect(refreshed.headers()["x-request-id"]).not.toBe(conflict.headers()["x-request-id"]);
  await expect(staleProposal.getByText("Esta proposta mudou. As alternativas restantes foram atualizadas.", { exact: true })).toBeVisible();
  await expect(staleProposal.getByText("Invalidada", { exact: true })).toBeVisible();
  const staleURL = staleProposal.url();
  expect(new URL(staleURL).searchParams.get("workflow")).toBe(conflict.headers()["x-correlation-id"]);
  await expect(staleProposal.getByText("ADOBE* CLOUD", { exact: true })).toBeVisible();
  await expect(staleProposal.getByText("-R$ 250,00", { exact: true }).first()).toBeVisible();
  await staleProposal.reload();
  await changeWorkspaceLanguage(staleProposal, "en");
  await expect(staleProposal.getByText("Invalidated", { exact: true })).toBeVisible();
  await expect(staleProposal.getByRole("heading", { name: "Original Match Evidence" })).toBeVisible();
  await expect(staleProposal).toHaveURL(staleURL);
  await staleProposal.getByRole("link", { name: "View reconciliation" }).click();
  await expect(staleProposal.getByRole("heading", { name: "Reconciliation detail" })).toBeVisible();
  await expect(staleProposal.getByText("CANVA* I045", { exact: true })).toBeVisible();

  await page.reload();
  await navigation.getByRole("link", { name: "Open Transactions", exact: true }).click();
  for (const reference of ["ERP-003", "CARD-004"]) {
    const row = page.getByRole("row").filter({ hasText: reference });
    await expect(row.getByText("R$1.00", { exact: true })).toBeVisible();
  }
});
