import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.describe("Browser language preference", () => {
  test.use({ locale: "pt-BR" });

  test("detects Portuguese, keeps a manual choice through sign-in and sign-out, and restores automatic detection", async ({ page }) => {
    const password = process.env.RECONCILIO_SMOKE_PASSWORD;
    if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

    const initialResponse = await page.goto(`${origin}/login`);
    expect(await initialResponse?.text()).toContain('lang="pt-BR"');
    await expect(page.getByRole("heading", { name: "Entrar", exact: true })).toBeVisible();
    await expect(page.getByRole("combobox", { name: "Idioma" })).toHaveValue("auto");

    await page.keyboard.press("Tab");
    await expect(page.getByRole("combobox", { name: "Idioma" })).toBeFocused();
    await expect(page.getByRole("combobox", { name: "Idioma" })).toHaveCSS("outline-style", "solid");
    await page.keyboard.press("Tab");
    await expect(page.getByLabel("Nome de usuário")).toBeFocused();
    await expect(page.getByLabel("Nome de usuário")).toHaveCSS("outline-style", "solid");

    await page.getByLabel("Nome de usuário").fill("admin");
    await page.getByLabel("Senha", { exact: true }).fill("incorrect password");
    await page.getByRole("button", { name: "Entrar", exact: true }).click();
    await expect(page.getByRole("region", { name: "Entrar" }).getByRole("alert"))
      .toHaveText("O nome de usuário ou a senha está incorreto.");

    await page.getByRole("combobox", { name: "Idioma" }).selectOption("en");
    await expect(page.getByRole("heading", { name: "Sign in", exact: true })).toBeVisible();
    await expect(page.getByRole("region", { name: "Sign in" }).getByRole("alert"))
      .toHaveText("The username or password is incorrect.");
    await expect(page.getByLabel("Username")).toHaveValue("admin");
    await expect(page).toHaveURL(`${origin}/login`);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");

    await page.reload();
    await expect(page.getByRole("combobox", { name: "Language" })).toHaveValue("en");
    await page.getByLabel("Username").fill("");
    await page.getByLabel("Password", { exact: true }).fill("");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page.getByRole("region", { name: "Sign in" }).getByRole("alert"))
      .toHaveText("Enter a valid username and password.");

    await page.getByLabel("Username").fill("admin");
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Reconciliation workspace" })).toBeVisible();

    await page.getByRole("combobox", { name: "Language" }).selectOption("pt-BR");
    await expect(page.getByRole("heading", { name: "Área de conciliação" })).toBeVisible();
    await expect(page).toHaveURL(`${origin}/workspace`);
    await page.getByRole("button", { name: "Sair", exact: true }).click();
    await expect(page.getByRole("combobox", { name: "Idioma" })).toHaveValue("pt-BR");

    const savedPreference = page.waitForResponse(response =>
      response.url() === `${origin}/api/preferences/language` && response.request().method() === "POST",
    );
    await page.getByRole("combobox", { name: "Idioma" }).selectOption("auto");
    expect((await savedPreference).status()).toBe(204);
    await expect(page.getByRole("combobox", { name: "Idioma" })).toHaveValue("auto");
    await page.reload();
    await expect(page.getByRole("combobox", { name: "Idioma" })).toHaveValue("auto");
  });
});

test("falls back to English and rejects invalid preference requests", async ({ request }) => {
  const response = await request.get(`${origin}/login`, { headers: { "Accept-Language": "fr-FR,de;q=0.8" } });
  expect(await response.text()).toContain('lang="en"');
  const regional = await request.get(`${origin}/login`, { headers: { "Accept-Language": "en-GB;q=0.5,pt-PT;q=0.9" } });
  expect(await regional.text()).toContain('lang="pt-BR"');

  const endpoint = `${origin}/api/preferences/language`;
  const crossOrigin = await request.post(endpoint, {
    headers: { Origin: "https://example.invalid" },
    data: { locale: "pt-BR" },
  });
  expect(crossOrigin.status()).toBe(403);

  const invalid = await request.post(endpoint, {
    headers: { Origin: origin },
    data: { locale: "fr-FR" },
  });
  expect(invalid.status()).toBe(400);
});
