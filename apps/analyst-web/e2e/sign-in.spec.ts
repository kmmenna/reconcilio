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
  await expect(page.getByText("Administrator", { exact: true })).toBeVisible();
  const session = (await context.cookies()).find((cookie) => cookie.name === "reconcilio_session");
  expect(session).toMatchObject({ httpOnly: true, sameSite: "Lax" });

  await page.reload();
  await expect(page.getByRole("heading", { name: "Reconciliation workspace" })).toBeVisible();

  const denied = await context.request.post(`${origin}/api/auth/logout`);
  expect(denied.status()).toBe(403);
  const crossOrigin = await context.request.post(`${origin}/api/auth/logout`, {
    headers: { Origin: "https://example.invalid" },
  });
  expect(crossOrigin.status()).toBe(403);
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  expect((await context.cookies()).some((cookie) => cookie.name === "reconcilio_session")).toBe(false);
  await page.goto(`${origin}/workspace`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();

  await context.addCookies([{ name: "reconcilio_session", value: "invalid-token", url: origin }]);
  await page.goto(`${origin}/workspace`);
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
});
