import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { cache } from "react";

import { readProfile, sessionCookie } from "@/lib/auth";

export const getWorkspaceToken = cache(async () => {
  const token = (await cookies()).get(sessionCookie)?.value;
  if (!token) redirect("/login");
  return token;
});

export const getWorkspaceProfile = cache(async () => {
  const profile = await readProfile(await getWorkspaceToken()).catch(() => undefined);
  if (profile === null) redirect("/login");
  return profile;
});
