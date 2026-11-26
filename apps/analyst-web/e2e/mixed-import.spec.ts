import { expect, test } from "@playwright/test";

import { selectFormOption } from "./form-select";
import { changeWorkspaceLanguage } from "./workspace-language";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US", colorScheme: "dark" });

test("manages the catalog and confirms a mixed CSV after refreshing a changed Source side", async ({ page }) => {
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  await page.goto(`${origin}/workspace`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  const navigation = page.getByRole("navigation", { name: "Workspace navigation" });
  await navigation.getByRole("link", { name: "Catalog", exact: true }).click();
  const accounts = page.getByRole("region", { name: "Financial Accounts" });
  const sources = page.getByRole("region", { name: "Sources", exact: true });
  await accounts.getByRole("button", { name: "Add Financial Account" }).click();
  await accounts.getByLabel("Name", { exact: true }).fill("Conta Operacional");
  await accounts.getByLabel("Code", { exact: true }).fill("operating-bank");
  await accounts.getByRole("button", { name: "Save" }).click();
  await expect(accounts.getByText("Conta Operacional", { exact: true })).toBeVisible();
  await expect(accounts.getByText("operating-bank", { exact: true })).toBeVisible();
  await accounts.getByRole("button", { name: "Edit Conta Operacional" }).click();
  await expect(accounts.getByLabel("Code", { exact: true })).toHaveCount(0);
  await accounts.getByLabel("Name", { exact: true }).fill("Conta Principal");
  await accounts.getByRole("button", { name: "Save" }).click();
  await expect(accounts.getByText("Conta Principal", { exact: true })).toBeVisible();
  await expect(accounts.getByText("operating-bank", { exact: true })).toBeVisible();

  await sources.getByRole("button", { name: "Add Source" }).click();
  await sources.getByLabel("Name", { exact: true }).fill("Sistema da Matriz");
  const side = sources.getByRole("combobox", { name: "Side", exact: true });
  await expect(side.locator("svg.lucide-log-in")).toBeVisible();
  await side.click();
  await expect(page.getByRole("option", { name: "External" }).locator("svg.lucide-log-out")).toBeVisible();
  await page.getByRole("option", { name: "Internal" }).click();
  await sources.getByRole("button", { name: "Save" }).click();
  await expect(sources.getByText("Sistema da Matriz", { exact: true })).toBeVisible();

  await navigation.getByRole("link", { name: "Import CSV", exact: true }).click();
  const fileInput = page.getByLabel("CSV file", { exact: true });
  const codeHelp = page.getByText("Use a registered Financial Account code", { exact: false });
  const source = page.getByRole("combobox", { name: "Source", exact: true });
  await expect(source).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await expect(page.getByLabel("Start date", { exact: true })).toHaveCount(0);
  await expect(page.getByLabel("End date", { exact: true })).toHaveCount(0);
  await expect(codeHelp).toHaveCount(0);
  await selectFormOption(source, "Sistema da Matriz");
  await selectFormOption(page.getByRole("combobox", { name: "Financial Account", exact: true }), "Use account codes from CSV");
  await expect(codeHelp).toBeVisible();
  expect(await fileInput.evaluate((input) => Boolean(input.compareDocumentPosition(document.getElementById("csv-account-help")!) & Node.DOCUMENT_POSITION_FOLLOWING))).toBe(true);
  const file = (code: string) => ({
    name: "mixed.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(
      "source_reference,payment_reference,transaction_date,amount,currency,description,financial_account\n" +
      "MIXED-001,,2026-11-02,-89.90,BRL,Lançamento externo,corporate-card\n" +
      `MIXED-001,,2026-11-03,240,JPY,Recebimento,${code}\n`,
    ),
  });
  await fileInput.setInputFiles({ name: "notes.txt", mimeType: "text/plain", buffer: Buffer.from("notes") });
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("Choose one CSV file.");
  const chooser = page.waitForEvent("filechooser");
  await fileInput.press("Space");
  const selected = file("missing-bank");
  await (await chooser).setFiles(selected);
  await expect(page.getByText("Choose one CSV file.")).toHaveCount(0);
  const size = new Intl.NumberFormat("en", { style: "unit", unit: "byte", unitDisplay: "long" }).format(selected.buffer.length);
  await expect(page.getByText(`mixed.csv · ${size}`, { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Preview CSV" }).click();
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Line 3 · Financial Account");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("missing-bank");
  await changeWorkspaceLanguage(page, "pt-BR");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Linha 3 · Conta financeira");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("O código missing-bank não está cadastrado.");
  await changeWorkspaceLanguage(page, "en");

  const dropped = file("operating-bank");
  const transfer = await page.evaluateHandle(({ name, mimeType, text }) => {
    const data = new DataTransfer();
    data.items.add(new File([text], name, { type: mimeType }));
    return data;
  }, { name: dropped.name, mimeType: dropped.mimeType, text: dropped.buffer.toString() });
  await fileInput.dispatchEvent("dragover", { dataTransfer: transfer });
  await fileInput.dispatchEvent("drop", { dataTransfer: transfer });
  await transfer.dispose();
  const droppedSize = new Intl.NumberFormat("en", { style: "unit", unit: "byte", unitDisplay: "long" }).format(dropped.buffer.length);
  await expect(page.getByText(`mixed.csv · ${droppedSize}`, { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Preview CSV" }).click();
  await expect(page.getByRole("heading", { name: "Import preview" })).toBeVisible();
  await expect(page.getByText("Sistema da Matriz", { exact: true })).toBeVisible();
  await expect(page.getByRole("row").filter({ hasText: "Lançamento externo" })).toContainText("corporate-card");
  await expect(page.getByRole("row").filter({ hasText: "Recebimento" })).toContainText("operating-bank");
  const coverage = page.getByRole("region", { name: "Import Coverage" });
  await expect(coverage.getByRole("listitem").filter({ hasText: "Corporate Card" })).toContainText("Nov 2, 2026");
  await expect(coverage.getByRole("listitem").filter({ hasText: "Conta Principal" })).toContainText("Nov 3, 2026");
  const originalURL = page.url();

  await page.goto(`${origin}/workspace/catalog`);
  const sourceRow = sources.getByRole("listitem").filter({ hasText: "Sistema da Matriz" });
  await sourceRow.getByRole("button", { name: "Edit Sistema da Matriz" }).click();
  await selectFormOption(sources.getByRole("combobox", { name: "Side", exact: true }), "External");
  await sources.getByRole("button", { name: "Save" }).click();
  await expect(sourceRow.getByRole("button", { name: "Edit Sistema da Matriz" })).toBeVisible();
  await expect(sourceRow.getByText("External", { exact: true })).toBeVisible();

  await page.goto(originalURL);
  await page.getByRole("button", { name: "Confirm import" }).click();
  await expect(page.getByRole("main").getByRole("alert")).toContainText("The Source side changed");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Internal");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("External");
  await changeWorkspaceLanguage(page, "pt-BR");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Interno");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Externo");
  await expect(page).toHaveURL(originalURL);

  await page.getByRole("button", { name: "Gerar nova prévia" }).click();
  await expect(page).not.toHaveURL(originalURL);
  await expect(page.getByRole("heading", { name: "Prévia da importação" })).toBeVisible();
  await expect(page.getByText("Externo", { exact: true })).toBeVisible();
  await expect(page.getByText("-R$ 89,90", { exact: true })).toBeVisible();
  const localizedCoverage = page.getByRole("region", { name: "Cobertura da importação" });
  await expect(localizedCoverage.getByRole("listitem").filter({ hasText: "Corporate Card" })).toContainText("2 de nov. de 2026");
  await expect(localizedCoverage.getByRole("listitem").filter({ hasText: "Conta Principal" })).toContainText("3 de nov. de 2026");
  await page.getByRole("button", { name: "Confirmar importação" }).click();
  await expect(page.getByRole("heading", { name: "Importação confirmada" })).toBeVisible();
  await expect(localizedCoverage.getByRole("listitem").filter({ hasText: "Corporate Card" })).toContainText("2 de nov. de 2026");
  await expect(localizedCoverage.getByRole("listitem").filter({ hasText: "Conta Principal" })).toContainText("3 de nov. de 2026");
  const jobs = page.getByRole("link", { name: "Acompanhar processamento" });
  await expect(jobs).toHaveCount(2);
  expect(await jobs.first().getAttribute("href")).not.toBe(await jobs.last().getAttribute("href"));

  await page.goto(originalURL);
  await expect(page.getByRole("heading", { name: "Prévia da importação" })).toBeVisible();
  await expect(page.getByText("Interno", { exact: true })).toBeVisible();

  await page.goto(`${origin}/workspace/catalog`);
  const portugueseSources = page.getByRole("region", { name: "Fontes" });
  await portugueseSources.getByRole("button", { name: "Editar Sistema da Matriz" }).click();
  await expect(portugueseSources.getByLabel("Lado", { exact: true })).toBeDisabled();
  await expect(portugueseSources.getByLabel("Nome", { exact: true })).toBeEditable();
});
