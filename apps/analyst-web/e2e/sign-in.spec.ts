import { expect, test } from "@playwright/test";

const origin = process.env.RECONCILIO_SMOKE_ORIGIN ?? "http://localhost:3100";

test("the local administrator signs in, reloads the workspace, and signs out", async ({ page, context, request }) => {
  const password = process.env.RECONCILIO_SMOKE_PASSWORD;
  if (!password) throw new Error("Run the smoke task or set RECONCILIO_SMOKE_PASSWORD.");

  await page.goto(origin);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  const fields = () => ({
    username: page.getByLabel("Username"),
    password: page.getByLabel("Password"),
    submit: page.getByRole("button", { name: "Sign in" }),
  });

  for (const value of [undefined, "https://example.invalid"]) {
    const response = await request.post(`${origin}/api/auth/login`, {
      data: { username: "admin", password },
      headers: value ? { Origin: value } : {},
    });
    expect(response.status()).toBe(403);
  }

  await fields().username.fill("admin");
  await fields().password.fill("incorrect password");
  await fields().submit.click();
  await expect(page.getByRole("region", { name: "Sign in" }).getByRole("alert"))
    .toHaveText("The username or password is incorrect.");

  await fields().password.fill(password);
  await fields().submit.click();
  await expect(page).toHaveURL(`${origin}/workspace/imports`);
  await expect(page.getByRole("heading", { name: "Import CSV" })).toBeVisible();
  const header = page.getByRole("banner");
  const account = header.getByRole("button", { name: "Administrator admin", exact: true });
  await expect(account.getByText("Administrator", { exact: true })).toBeVisible();
  await expect(account.getByText("admin", { exact: true })).toBeVisible();
  await expect(account).toHaveCSS("border-top-width", "0px");
  await expect(account).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  const main = page.getByRole("main");
  await expect(main.getByText("Administrator", { exact: true })).toHaveCount(0);
  await expect(main.getByText("admin", { exact: true })).toHaveCount(0);
  await expect(main.getByRole("link", { name: "My profile" })).toHaveCount(0);
  const session = (await context.cookies()).find((cookie) => cookie.name === "reconcilio_session");
  expect(session).toMatchObject({ httpOnly: true, sameSite: "Lax" });

  await page.goto(`${origin}/workspace`);
  await expect(page).toHaveURL(`${origin}/workspace/imports`);
  await expect(page.getByRole("heading", { name: "Import CSV" })).toBeVisible();

  const navigation = page.getByRole("navigation", { name: "Workspace navigation" });
  const destinations = [
    ["Import CSV", "imports"],
    ["Reconciliations", "reconciliations"],
    ["Review queue", "review"],
    ["Open Transactions", "transactions"],
    ["Catalog", "catalog"],
  ];
  await test.step("switch work areas from the desktop sidebar", async () => {
    await expect(navigation.getByRole("link")).toHaveText(destinations.map(([name]) => name));
    const [sidebarBounds, contentBounds] = await Promise.all([navigation.boundingBox(), main.boundingBox()]);
    expect(sidebarBounds!.x + sidebarBounds!.width).toBeLessThanOrEqual(contentBounds!.x);

    await account.focus();
    await page.keyboard.press("Tab");
    const importLink = navigation.getByRole("link", { name: "Import CSV" });
    await expect(importLink).toBeFocused();
    await expect(importLink).toHaveCSS("outline-style", "solid");

    for (const [name, path] of destinations) {
      const link = navigation.getByRole("link", { name, exact: true });
      await link.click();
      await expect(page).toHaveURL(`${origin}/workspace/${path}`);
      await expect(main.getByRole("heading", { name, exact: true })).toBeVisible();
      await expect(navigation.getByRole("link")).toHaveCount(destinations.length);
      await expect(navigation.locator('[aria-current="location"]')).toHaveText(name);
    }

    await page.reload();
    await expect(navigation.locator('[aria-current="location"]')).toHaveText("Catalog");
  });

  await test.step("choose a work area with the narrow-screen menu by keyboard", async () => {
    await page.setViewportSize({ width: 320, height: 720 });
    const menu = navigation.getByRole("button", { name: "Menu", exact: true });
    await expect(menu).toHaveAttribute("aria-expanded", "false");
    await expect(navigation.getByRole("link")).toHaveCount(0);

    await account.focus();
    await page.keyboard.press("Tab");
    await expect(menu).toBeFocused();
    await expect(menu).toHaveCSS("outline-style", "solid");
    await page.keyboard.press("Enter");
    await expect(menu).toHaveAttribute("aria-expanded", "true");
    await expect(navigation.getByRole("link")).toHaveText(destinations.map(([name]) => name));
    await expect(navigation.locator('[aria-current="location"]')).toHaveText("Catalog");
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);

    for (const [name] of destinations.slice(0, 3)) {
      await page.keyboard.press("Tab");
      const link = navigation.getByRole("link", { name, exact: true });
      await expect(link).toBeFocused();
      await expect(link).toHaveCSS("outline-style", "solid");
    }
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(`${origin}/workspace/review`);
    await expect(main.getByRole("heading", { name: "Review queue" })).toBeVisible();
    await expect(menu).toHaveAttribute("aria-expanded", "false");
    await expect(navigation.getByRole("link")).toHaveCount(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320);

    await page.setViewportSize({ width: 1280, height: 720 });
  });

  const profileAction = page.getByRole("menuitem", { name: "My profile" });
  await expect(account).toHaveAttribute("aria-expanded", "false");
  await header.getByRole("link", { name: "Create user", exact: true }).focus();
  await page.keyboard.press("Tab");
  await expect(account).toBeFocused();
  await expect(account).toHaveCSS("outline-style", "solid");
  await page.keyboard.press("Enter");
  await expect(account).toHaveAttribute("aria-expanded", "true");
  await expect(profileAction).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await expect(page.getByRole("menuitem", { name: "Sign out" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(account).toBeFocused();
  await expect(account).toHaveCSS("outline-style", "solid");
  await expect(account).toHaveAttribute("aria-expanded", "false");

  await account.press("Enter");
  await expect(profileAction).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(`${origin}/workspace/profile`);
  await expect(navigation.getByRole("link")).toHaveCount(destinations.length);
  await expect(navigation.locator('[aria-current="location"]')).toHaveCount(0);
  await expect(account).toBeVisible();
  await header.getByRole("link", { name: "Reconcilio" }).click();
  await expect(page).toHaveURL(`${origin}/workspace/imports`);

  const denied = await context.request.post(`${origin}/api/auth/logout`);
  expect(denied.status()).toBe(403);
  const crossOrigin = await context.request.post(`${origin}/api/auth/logout`, {
    headers: { Origin: "https://example.invalid" },
  });
  expect(crossOrigin.status()).toBe(403);

  const logout = Promise.withResolvers<void>();
  await page.route(`${origin}/api/auth/logout`, async (route) => {
    await logout.promise;
    await route.continue();
  }, { times: 1 });
  const signOutAction = page.getByRole("menuitem", { name: "Sign out", exact: true });
  await account.press("Enter");
  await expect(profileAction).toBeFocused();
  await page.keyboard.press("ArrowDown");
  try {
    await signOutAction.press("Enter");
    await expect(page.getByRole("menuitem", { name: "Signing out…" })).toBeDisabled();
  } finally {
    logout.resolve();
  }
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  expect((await context.cookies()).some((cookie) => cookie.name === "reconcilio_session")).toBe(false);
  await page.goto(`${origin}/workspace`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();

  await page.goto(`${origin}/workspace/reconciliations`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();

  await context.addCookies([{ name: "reconcilio_session", value: "invalid-token", url: origin }]);
  await page.goto(`${origin}/workspace`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
});
