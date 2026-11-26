import { expect, test } from "@playwright/test";

import { selectFormOption } from "./form-select";
import { changeWorkspaceLanguage } from "./workspace-language";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.use({ locale: "en-US", timezoneId: "Asia/Tokyo" });

test("reprocesses the selected Financial Account and keeps the summary link across languages", async ({ page }, testInfo) => {
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  await page.goto(`${origin}/workspace/reconciliations`);
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  const navigation = page.getByRole("navigation", { name: "Workspace navigation" });
  await navigation.getByRole("link", { name: "Reconciliations", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Reconciliations" })).toBeVisible();

  const account = { name: "Reprocessing Test Account", code: "reprocessing-smoke" };
  const created = await page.request.post(`${origin}/api/catalog/financial-accounts`, {
    headers: { Origin: origin },
    data: account,
  });
  expect(created.status()).toBe(201);
  const { id: accountID } = await created.json() as { id: string };
  await page.goto(`${origin}/workspace/reconciliations`);

  const reprocess = page.getByRole("button", { name: "Reprocess account" });
  await expect(reprocess).toBeDisabled();
  await selectFormOption(page.getByRole("combobox", { name: "Financial Account" }), account.name);
  const denied = await page.request.post(`${origin}/api/financial-accounts/${accountID}/reprocess`, {
    headers: { Origin: "https://example.invalid" },
  });
  expect(denied.status()).toBe(403);
  await expect(reprocess).toBeEnabled();
  await page.screenshot({ path: testInfo.outputPath("desktop-reprocessing.png") });

  const queued = page.waitForResponse((response) => response.request().method() === "POST" &&
    response.url().endsWith("/reprocess"));
  await reprocess.click();
  expect((await queued).status()).toBe(202);
  await expect(page).toHaveURL(url => /^\/workspace\/jobs\/\d+$/.test(url.pathname));
  const jobURL = page.url();
  await expect(page.getByRole("status")).toHaveText("Completed", { timeout: 15_000 });
  await expect(page.getByText(account.name, { exact: true })).toBeVisible();
  await expect(page.getByText("No automatic Reconciliations", { exact: true })).toBeVisible();
  await expect(page.getByText("No new Match Proposals", { exact: true })).toBeVisible();
  const completedAt = page.locator("time");
  const timestamp = await completedAt.getAttribute("datetime");
  await expect(completedAt).toHaveText(/GMT\+9$/);
  const englishTime = await completedAt.textContent();
  if (!timestamp || !englishTime) throw new Error("The summary is missing its completion time.");
  await page.screenshot({ path: testInfo.outputPath("desktop-summary.png") });

  await page.setViewportSize({ width: 390, height: 844 });
  await changeWorkspaceLanguage(page, "pt-BR");
  await expect(page.getByRole("status")).toHaveText("Concluído");
  await expect(page).toHaveURL(jobURL);
  await expect(completedAt).toHaveAttribute("datetime", timestamp);
  await expect(completedAt).toHaveText(/GMT\+9$/);
  await expect(completedAt).not.toHaveText(englishTime);
  const tokyoClock = new Date(new Date(timestamp).getTime() + 9 * 60 * 60 * 1000);
  const tokyoDate = new Intl.DateTimeFormat("pt-BR", { dateStyle: "medium", timeZone: "UTC" }).format(tokyoClock);
  await expect(completedAt).toContainText(tokyoDate);
  await expect(completedAt).toContainText(tokyoClock.toISOString().slice(11, 16));
  await expect(page.getByText("Nenhuma conciliação automática", { exact: true })).toBeVisible();
  await expect(page.getByText("Nenhuma nova proposta de conciliação", { exact: true })).toBeVisible();
  await page.reload();
  await expect(completedAt).toHaveText(/GMT\+9$/);
  await expect(page.getByRole("status")).toHaveText("Concluído");
  await expect(page).toHaveURL(jobURL);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
  await page.screenshot({ path: testInfo.outputPath("mobile-summary.png") });
  await page.getByRole("link", { name: "Ver conciliações" }).click();
  await expect(page.getByRole("button", { name: "Reprocessar conta" })).toBeEnabled();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
  await page.screenshot({ path: testInfo.outputPath("mobile-reprocessing.png") });
});
