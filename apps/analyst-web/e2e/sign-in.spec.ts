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
  await expect(page.getByRole("heading", { name: "Reconciliation workspace" })).toBeVisible();
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

  await page.reload();
  await expect(page.getByRole("heading", { name: "Reconciliation workspace" })).toBeVisible();

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
  await expect(account).toBeVisible();
  await header.getByRole("link", { name: "Reconcilio" }).click();
  await expect(page).toHaveURL(`${origin}/workspace`);

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

  await context.addCookies([{ name: "reconcilio_session", value: "invalid-token", url: origin }]);
  await page.goto(`${origin}/workspace`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
});
