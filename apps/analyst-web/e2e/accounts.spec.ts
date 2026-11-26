import { randomUUID } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

async function signIn(page: Page, username: string, password: string) {
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
}

test("creates an analyst and lets them update their own profile and sign-in credentials", async ({ page, context }) => {
  const adminPassword = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!adminPassword) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");
  const username = `Analista.${randomUUID()}`;
  const displayName = "Cecília de Sá";
  const initialPassword = "initial-password";
  const newPassword = "changed-password";
  const header = page.getByRole("banner");
  const accountMenu = header.getByRole("button", { name: `${displayName} ${username}`, exact: true });
  const signOut = page.getByRole("menuitem", { name: "Sign out", exact: true });

  await page.goto(`${origin}/login`);
  await signIn(page, "admin", adminPassword);
  await page.getByRole("link", { name: "Create user" }).click();

  const account = { username, display_name: displayName, password: initialPassword };
  const missingOrigin = await context.request.post(`${origin}/api/users`, { data: account });
  expect(missingOrigin.status()).toBe(403);

  const fillAccount = async () => {
    await page.getByLabel("Username", { exact: true }).fill(username);
    await page.getByLabel("Display name", { exact: true }).fill(displayName);
    await page.getByLabel("Password", { exact: true }).fill(initialPassword);
    await page.getByRole("button", { name: "Create user", exact: true }).click();
  };
  await fillAccount();
  await expect(page.getByRole("status")).toHaveText(`Account for ${displayName} (${username}) created.`);
  await fillAccount();
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("This username is already in use. Choose a different one.");

  await page.getByRole("combobox", { name: "Language" }).selectOption("pt-BR");
  await expect(page.getByRole("heading", { name: "Criar usuário" })).toBeVisible();
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("Este nome de usuário já está em uso. Escolha outro.");
  await expect(page.getByLabel("Nome de usuário")).toHaveValue(username);
  await expect(page).toHaveURL(`${origin}/workspace/users/new`);
  await page.getByRole("combobox", { name: "Idioma" }).selectOption("en");

  await expect(page.getByRole("heading", { name: "Create user" })).toBeVisible();
  await header.getByRole("button", { name: "Administrator admin", exact: true }).click();
  await signOut.click();
  await expect(page).toHaveURL(`${origin}/login`);
  await page.reload();
  await signIn(page, username, initialPassword);
  await expect(page.getByRole("heading", { name: "Reconciliation workspace" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Create user" })).toHaveCount(0);
  await page.goto(`${origin}/workspace/users/new`);
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("Ask an administrator to create the account.");
  await page.getByRole("link", { name: "Back to workspace" }).click();

  await page.setViewportSize({ width: 320, height: 720 });
  await accountMenu.click();
  const nameLine = accountMenu.getByText(displayName, { exact: true });
  const usernameLine = accountMenu.getByText(username, { exact: true });
  await expect(nameLine).toBeInViewport({ ratio: 1 });
  await expect(usernameLine).toBeInViewport({ ratio: 1 });
  expect((await usernameLine.boundingBox())!.y).toBeGreaterThan((await nameLine.boundingBox())!.y);
  await expect(page.getByRole("menu")).toBeInViewport({ ratio: 1 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);
  await page.getByRole("menuitem", { name: "My profile" }).click();

  await expect(page.getByLabel("Display name")).toHaveValue(displayName);
  await page.getByLabel("Display name").fill("Cecília Sá Costa");
  await page.getByRole("button", { name: "Save display name" }).click();
  await expect(page.getByText("Display name saved.", { exact: true })).toBeVisible();
  const updatedAccountMenu = header.getByRole("button", { name: `Cecília Sá Costa ${username}`, exact: true });
  await expect(updatedAccountMenu).toBeVisible();

  await page.getByLabel("Current password").fill("incorrect-password");
  await page.getByLabel("New password").fill(newPassword);
  await page.getByRole("button", { name: "Change password" }).click();
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("The current password is incorrect. Check it and try again.");
  await page.getByLabel("Current password").fill(initialPassword);
  await page.getByRole("button", { name: "Change password" }).click();
  await expect(page.getByText("Password changed.", { exact: true })).toBeVisible();
  await expect(page.getByLabel("Current password")).toBeEmpty();
  await expect(page.getByLabel("New password")).toBeEmpty();

  await updatedAccountMenu.click();
  await signOut.click();
  await expect(page).toHaveURL(`${origin}/login`);
  await page.reload();
  await signIn(page, username, initialPassword);
  await expect(page.getByRole("main").getByRole("alert")).toHaveText("The username or password is incorrect.");
  await signIn(page, username, newPassword);
  await expect(updatedAccountMenu).toBeVisible();
});
