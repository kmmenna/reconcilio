import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";
const provider = process.env.RECONCILIO_SMOKE_RANKING_URL;

test.use({ locale: "en-US" });

test("keeps the recorded bilingual ranking on shared links without a provider call", async ({ browser, page, request }) => {
  test.skip(!provider, "Run the smoke task with RECONCILIO_SMOKE_RANKING=true.");
  test.setTimeout(60_000);
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  async function providerCalls(): Promise<number> {
    const response = await request.get(`${provider}/requests`);
    expect(response.ok()).toBeTruthy();
    return (await response.json()).submissions;
  }

  await page.goto(`${origin}/workspace/imports`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByLabel("Financial Account")).toBeVisible();

  const account = await page.getByLabel("Financial Account").locator("option").filter({ hasText: "Corporate Card" }).getAttribute("value");
  if (!account) throw new Error("The Corporate Card demo account is missing.");
  for (const [source, fixture] of [["Demo ERP", "internal.csv"], ["Demo Card Statement", "external.csv"]]) {
    const sourceID = await page.getByLabel("Source", { exact: true }).locator("option").filter({ hasText: source }).getAttribute("value");
    if (!sourceID) throw new Error(`Demo source ${source} is missing.`);
    const preview = await page.request.post(`${origin}/api/imports`, {
      headers: { Origin: origin },
      multipart: {
        source_id: sourceID,
        financial_account_id: account,
        period_start: "2026-11-16",
        period_end: "2026-11-22",
        file: { name: fixture, mimeType: "text/csv", buffer: await readFile(resolve(__dirname, "../../../examples/demo", fixture)) },
      },
    });
    expect(preview.status()).toBe(201);
    const { id } = await preview.json();
    const confirmation = await page.request.post(`${origin}/api/imports/${id}/confirm`, { headers: { Origin: origin } });
    expect(confirmation.status()).toBe(200);
    const { jobs } = await confirmation.json();
    await expect.poll(async () => {
      const job = await page.request.get(`${origin}/api/jobs/${jobs[0].job_id}`);
      return (await job.json()).status;
    }, { timeout: 15_000 }).toBe("completed");
  }

  await page.goto(`${origin}/workspace/review`);
  await page.getByRole("link", { name: "Compare alternatives" }).click();
  await page.waitForURL(/\/workspace\/review\/[0-9a-f-]+$/);
  const groupURL = page.url();
  const ranked = page.locator("[data-ranking-evaluation-id]");
  const canva = page.getByRole("article", { name: "CANVA* I045" });
  const adobe = page.getByRole("article", { name: "ADOBE* CLOUD" });
  await expect(page.getByRole("article").first()).toHaveAccessibleName("CANVA* I045");
  await expect(canva.getByText("Rank 1 · Score 93/100")).toBeVisible();
  await expect(adobe.getByText("Rank 2 · Score 24/100")).toBeVisible();
  await expect(canva.getByText("Both descriptions point to the same Canva subscription.")).toBeVisible();
  await expect(adobe.getByText("The statement names a different merchant.")).toBeVisible();
  const evaluation = await ranked.first().getAttribute("data-ranking-evaluation-id");
  expect(evaluation).toMatch(/^[0-9a-f-]{36}$/);
  await expect(ranked.last()).toHaveAttribute("data-ranking-evaluation-id", evaluation!);
  expect(await providerCalls()).toBe(1);
  await page.screenshot({ path: test.info().outputPath("review-en.png"), fullPage: true });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(canva.getByText("As descrições mencionam a mesma assinatura do Canva.")).toBeVisible();
  await expect(adobe.getByText("O lançamento menciona um fornecedor diferente.")).toBeVisible();
  await expect(page.getByRole("article").first()).toHaveAccessibleName("CANVA* I045");
  await expect(canva.getByText("Posição 1 · Pontuação 93/100")).toBeVisible();
  await expect(adobe.getByText("Posição 2 · Pontuação 24/100")).toBeVisible();
  await expect(ranked.first()).toHaveAttribute("data-ranking-evaluation-id", evaluation!);
  await expect(ranked.last()).toHaveAttribute("data-ranking-evaluation-id", evaluation!);
  await expect(page).toHaveURL(groupURL);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
  await page.screenshot({ path: test.info().outputPath("review-pt-BR.png"), fullPage: true });

  await canva.getByRole("button", { name: "Confirmar conciliação" }).click();
  await expect(page.getByRole("heading", { name: "Avaliação da IA registrada" })).toBeVisible();
  await expect(ranked).toHaveAttribute("data-ranking-evaluation-id", evaluation!);
  await expect(ranked.getByText("As descrições mencionam a mesma assinatura do Canva.")).toBeVisible();
  const proposalURL = page.url();
  await page.getByLabel("Idioma").selectOption("en");
  await expect(ranked.getByText("Both descriptions point to the same Canva subscription.")).toBeVisible();
  await expect(ranked.getByText("Rank 1 · Score 93/100")).toBeVisible();
  await expect(page).toHaveURL(proposalURL);

  await page.getByRole("link", { name: "View reconciliation", exact: true }).click();
  await page.waitForURL(/\/workspace\/reconciliations\/[0-9a-f-]+$/);
  await expect(page.getByRole("heading", { name: "Recorded AI assessment" })).toBeVisible();
  const reconciliationURL = page.url();
  await expect(ranked).toHaveAttribute("data-ranking-evaluation-id", evaluation!);
  await expect(ranked.getByText("Both descriptions point to the same Canva subscription.")).toBeVisible();
  await page.getByLabel("Language").selectOption("pt-BR");
  await expect(ranked.getByText("As descrições mencionam a mesma assinatura do Canva.")).toBeVisible();
  await expect(ranked.getByText("Posição 1 · Pontuação 93/100")).toBeVisible();
  await expect(ranked).toHaveAttribute("data-ranking-evaluation-id", evaluation!);
  await expect(page).toHaveURL(reconciliationURL);
  await page.screenshot({ path: test.info().outputPath("history-pt-BR.png"), fullPage: true });

  for (const reader of [
    {
      locale: "en-US", language: "en", username: "Username", passwordLabel: "Password", signIn: "Sign in",
      assessment: "Recorded AI assessment", score: "Rank 1 · Score 93/100",
      explanation: "Both descriptions point to the same Canva subscription.",
    },
    {
      locale: "pt-BR", language: "pt-BR", username: "Nome de usuário", passwordLabel: "Senha", signIn: "Entrar",
      assessment: "Avaliação da IA registrada", score: "Posição 1 · Pontuação 93/100",
      explanation: "As descrições mencionam a mesma assinatura do Canva.",
    },
  ]) {
    const readerContext = await browser.newContext({ locale: reader.locale });
    try {
      const sharedPage = await readerContext.newPage();
      await sharedPage.goto(`${origin}/login`);
      await sharedPage.getByLabel(reader.username).fill("admin");
      await sharedPage.getByLabel(reader.passwordLabel).fill(password);
      await sharedPage.getByRole("button", { name: reader.signIn, exact: true }).click();
      await expect(sharedPage).toHaveURL(`${origin}/workspace/imports`);

      for (const sharedURL of [proposalURL, reconciliationURL]) {
        await sharedPage.goto(sharedURL);
        await expect(sharedPage).toHaveURL(sharedURL);
        await expect(sharedPage.locator("html")).toHaveAttribute("lang", reader.language);
        await expect(sharedPage.getByRole("heading", { name: reader.assessment })).toBeVisible();
        const recorded = sharedPage.locator("[data-ranking-evaluation-id]");
        await expect(recorded).toHaveAttribute("data-ranking-evaluation-id", evaluation!);
        await expect(recorded.getByText(reader.score)).toBeVisible();
        await expect(recorded.getByText(reader.explanation)).toBeVisible();
      }
    } finally {
      await readerContext.close();
    }
  }

  expect(await providerCalls()).toBe(1);
});
