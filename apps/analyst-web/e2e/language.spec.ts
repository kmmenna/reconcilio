import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test.describe("Browser language preference", () => {
  test.use({ locale: "pt-BR" });

  test("detects Portuguese, keeps a manual choice through sign-in and sign-out, and restores automatic detection", async ({ page, context }) => {
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

    const footer = page.getByRole("contentinfo");
    const workspaceLanguage = footer.getByRole("button", { name: /^(Language|Idioma)$/ });
    const languageMenu = page.getByRole("menu", { name: /^(Language|Idioma)$/ });
    await test.step("keep the footer reachable on short pages and narrow screens", async () => {
      await page.setViewportSize({ width: 1280, height: 900 });
      await page.getByRole("banner").getByRole("link", { name: "Create user", exact: true }).click();
      const submit = page.getByRole("main").getByRole("button", { name: "Create user", exact: true });
      await expect(submit).toBeVisible();
      await expect.poll(() => footer.evaluate((element) => Math.round(element.getBoundingClientRect().bottom - window.innerHeight)))
        .toBe(0);

      await page.setViewportSize({ width: 320, height: 240 });
      const main = page.getByRole("main");
      const mainBottom = await main.evaluate((element) => element.getBoundingClientRect().bottom);
      expect(mainBottom).toBeGreaterThan(240);
      expect(await footer.evaluate((element) => element.getBoundingClientRect().top)).toBeGreaterThanOrEqual(mainBottom);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
      await submit.focus();
      await page.keyboard.press("Tab");
      await expect(workspaceLanguage).toBeFocused();
      await expect(workspaceLanguage).toBeInViewport();
      await expect(workspaceLanguage).toHaveCSS("outline-style", "solid");
      await expect(workspaceLanguage).toHaveAttribute("aria-expanded", "false");
      await page.keyboard.press("Enter");
      await expect(workspaceLanguage).toHaveAttribute("aria-expanded", "true");
      await expect(languageMenu).toBeInViewport({ ratio: 1 });
      await expect(languageMenu.getByRole("menuitemradio", { name: "Automatic" })).toBeFocused();
      await page.keyboard.press("End");
      await expect(languageMenu.getByRole("menuitemradio", { name: "English", checked: true })).toBeFocused();
      await page.keyboard.press("Escape");
      await expect(workspaceLanguage).toBeFocused();
      await expect(workspaceLanguage).toHaveAttribute("aria-expanded", "false");
      await page.setViewportSize({ width: 1280, height: 720 });
    });

    await page.goto(`${origin}/workspace/transactions`);
    await expect(page.getByRole("banner").getByRole("button", { name: "Language", exact: true })).toHaveCount(0);
    await expect(footer.getByText("Reconcilio · Transaction reconciliation")).toBeVisible();

    await test.step("restore browser language from the workspace footer", async () => {
      await workspaceLanguage.press("Enter");
      await expect(languageMenu.getByRole("menuitemradio")).toHaveText(["Automatic", "Português (Brasil)", "English"]);
      await expect(languageMenu.getByRole("menuitemradio", { name: "English", checked: true })).toBeVisible();
      await expect(languageMenu.getByRole("menuitemradio", { name: "Automatic" })).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(page.getByRole("heading", { name: "Transações em aberto" })).toBeVisible();
      await expect(page).toHaveURL(`${origin}/workspace/transactions`);
      await expect(page.locator("html")).toHaveAttribute("lang", "pt-BR");
      await expect(footer.getByRole("button", { name: "Idioma", exact: true })).toBeFocused();

      await page.reload();
      await expect(page).toHaveURL(`${origin}/workspace/transactions`);
      await expect(page.getByRole("heading", { name: "Transações em aberto" })).toBeVisible();
      await workspaceLanguage.click();
      await expect(languageMenu.getByRole("menuitemradio", { name: "Automático", checked: true })).toBeVisible();
    });

    await test.step("report a failed language change and keep the saved preference", async () => {
      await page.keyboard.press("Escape");
      await expect(workspaceLanguage).toBeFocused();
      await page.setViewportSize({ width: 320, height: 240 });
      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
      await workspaceLanguage.click();
      await context.setOffline(true);
      await languageMenu.getByRole("menuitemradio", { name: "English", exact: true }).click();
      const alert = footer.getByRole("alert");
      await expect(alert).toHaveText("Não foi possível alterar o idioma. Tente novamente.");
      await expect(alert).toBeInViewport({ ratio: 1 });
      await expect(workspaceLanguage).toBeFocused();
      await expect(page.locator("html")).toHaveAttribute("lang", "pt-BR");
      await context.setOffline(false);
      await page.setViewportSize({ width: 1280, height: 720 });
      await workspaceLanguage.click();
      await expect(languageMenu.getByRole("menuitemradio", { name: "Automático", checked: true })).toBeVisible();
    });

    await languageMenu.getByRole("menuitemradio", { name: "Português (Brasil)", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Transações em aberto" })).toBeVisible();
    await expect(footer.getByText("Reconcilio · Conciliação de transações")).toBeVisible();
    await expect(footer.getByRole("alert")).toHaveCount(0);
    await expect(page).toHaveURL(`${origin}/workspace/transactions`);
    await expect(workspaceLanguage).toHaveAttribute("aria-expanded", "false");
    await workspaceLanguage.click();
    await expect(languageMenu.getByRole("menuitemradio", { name: "Português (Brasil)", checked: true })).toBeEnabled();
    await page.reload();
    await workspaceLanguage.click();
    await expect(languageMenu.getByRole("menuitemradio", { name: "Português (Brasil)", checked: true })).toBeVisible();
    await page.keyboard.press("Escape");

    await page.getByRole("banner").getByRole("button", { name: "Administrator admin", exact: true }).click();
    await expect(page.getByRole("menuitem", { name: "Meu perfil", exact: true })).toBeVisible();
    const signOut = page.getByRole("menuitem", { name: "Sair", exact: true });
    await context.setOffline(true);
    await signOut.click();
    await expect(page.getByRole("menu").getByRole("alert")).toHaveText("Não foi possível sair. Tente novamente.");
    await context.setOffline(false);
    await signOut.click();
    await expect(page.getByRole("heading", { name: "Entrar", exact: true })).toBeVisible();
    await expect(page.getByRole("combobox", { name: "Idioma" })).toHaveValue("pt-BR");

    await page.getByRole("combobox", { name: "Idioma" }).selectOption("en");
    await expect(page.getByRole("heading", { name: "Sign in", exact: true })).toBeVisible();
    await page.getByRole("combobox", { name: "Language" }).selectOption("auto");
    await expect(page.getByRole("heading", { name: "Entrar", exact: true })).toBeVisible();
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
