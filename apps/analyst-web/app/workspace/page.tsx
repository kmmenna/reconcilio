import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { LanguageControl } from "@/components/language/language-control";
import { readProfile, sessionCookie } from "@/lib/auth";
import { SignOutButton } from "./sign-out-button";

export default async function WorkspacePage() {
  const token = (await cookies()).get(sessionCookie)?.value;
  if (!token) redirect("/login");

  const profile = await readProfile(token).catch(() => undefined);
  if (profile === null) redirect("/login");

  const t = await getTranslations("Workspace");
  return (
    <>
      <header className="flex min-h-[4.75rem] flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-border bg-card px-[max(1.3rem,5vw)] py-3">
        <span className="wordmark">Reconcilio</span>
        <div className="flex min-w-0 flex-wrap items-start gap-x-2 gap-y-2 desktop:gap-x-4">
          <LanguageControl />
          {profile && <SignOutButton />}
        </div>
      </header>
      <main className="workspace">
        <h1 className="page-title">{profile ? t("title") : t("unavailable")}</h1>
        {profile ? (
          <dl className="mt-8 grid max-w-xl grid-cols-2 gap-5 border-y border-border py-5">
            <div className="min-w-0">
              <dt className="text-sm text-muted-foreground">{t("signedInAs")}</dt>
              <dd className="mt-1 font-semibold wrap-anywhere">{profile.display_name}</dd>
            </div>
            <div className="min-w-0">
              <dt className="text-sm text-muted-foreground">{t("username")}</dt>
              <dd className="mt-1 font-semibold wrap-anywhere">{profile.username}</dd>
            </div>
          </dl>
        ) : (
          <p className="mt-6 text-destructive" role="alert">{t("sessionUnavailable")}</p>
        )}
      </main>
    </>
  );
}
