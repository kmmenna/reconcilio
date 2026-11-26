import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US" });

test("manages the catalog and confirms a mixed CSV after refreshing a changed Source side", async ({ page }) => {
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  await page.goto(`${origin}/workspace`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("link", { name: "Catalog", exact: true }).click();
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
  await sources.getByLabel("Side", { exact: true }).selectOption("internal");
  await sources.getByRole("button", { name: "Save" }).click();
  await expect(sources.getByText("Sistema da Matriz", { exact: true })).toBeVisible();

  await page.getByRole("link", { name: "Import CSV" }).click();
  await page.getByLabel("Source", { exact: true }).selectOption({ label: "Sistema da Matriz" });
  await page.getByLabel("Financial Account", { exact: true }).selectOption({ label: "Use account codes from CSV" });
  await page.getByLabel("Start date").fill("11/1/2026");
  await page.getByLabel("End date").fill("11/5/2026");
  const file = (code: string) => ({
    name: "mixed.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(
      "source_reference,payment_reference,transaction_date,amount,currency,description,financial_account\n" +
      "MIXED-001,,2026-11-02,-89.90,BRL,Lançamento externo,corporate-card\n" +
      `MIXED-001,,2026-11-03,240,JPY,Recebimento,${code}\n`,
    ),
  });
  await page.getByLabel("CSV file").setInputFiles(file("missing-bank"));
  await page.getByRole("button", { name: "Preview CSV" }).click();
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Line 3 · Financial Account");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("missing-bank");
  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Linha 3 · Conta financeira");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("O código missing-bank não está cadastrado.");
  await page.getByLabel("Idioma").selectOption("en");

  await page.getByLabel("CSV file").setInputFiles(file("operating-bank"));
  await page.getByRole("button", { name: "Preview CSV" }).click();
  await expect(page.getByRole("heading", { name: "Import preview" })).toBeVisible();
  await expect(page.getByText("Sistema da Matriz", { exact: true })).toBeVisible();
  await expect(page.getByRole("row").filter({ hasText: "Lançamento externo" })).toContainText("corporate-card");
  await expect(page.getByRole("row").filter({ hasText: "Recebimento" })).toContainText("operating-bank");
  const originalURL = page.url();

  await page.goto(`${origin}/workspace/catalog`);
  const sourceRow = sources.getByRole("listitem").filter({ hasText: "Sistema da Matriz" });
  await sourceRow.getByRole("button", { name: "Edit Sistema da Matriz" }).click();
  await sources.getByLabel("Side", { exact: true }).selectOption("external");
  await sources.getByRole("button", { name: "Save" }).click();
  await expect(sourceRow.getByText("External", { exact: true })).toBeVisible();

  await page.goto(originalURL);
  await page.getByRole("button", { name: "Confirm import" }).click();
  await expect(page.getByRole("main").getByRole("alert")).toContainText("The Source side changed");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Internal");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("External");
  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Interno");
  await expect(page.getByRole("main").getByRole("alert")).toContainText("Externo");
  await expect(page).toHaveURL(originalURL);

  await page.getByRole("button", { name: "Gerar nova prévia" }).click();
  await expect(page).not.toHaveURL(originalURL);
  await expect(page.getByRole("heading", { name: "Prévia da importação" })).toBeVisible();
  await expect(page.getByText("Externo", { exact: true })).toBeVisible();
  await expect(page.getByText("-R$ 89,90", { exact: true })).toBeVisible();
  await expect(page.getByText("2 de nov. de 2026", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Confirmar importação" }).click();
  await expect(page.getByRole("heading", { name: "Importação confirmada" })).toBeVisible();
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
