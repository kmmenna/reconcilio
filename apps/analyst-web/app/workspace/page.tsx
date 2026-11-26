import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AccountLinks } from "@/components/accounts/account-links";
import { Button } from "@/components/ui/button";
import { getWorkspaceProfile } from "@/lib/workspace";

export default async function WorkspacePage() {
  const [profile, t, imports, reconciliation, review] = await Promise.all([
    getWorkspaceProfile(),
    getTranslations("Workspace"),
    getTranslations("Imports"),
    getTranslations("Reconciliation"),
    getTranslations("Review"),
  ]);

  return (
    <main className="workspace">
      <h1 className="page-title">{profile ? t("title") : t("unavailable")}</h1>
      {profile ? (
        <>
          <dl className="mt-8 grid max-w-xl grid-cols-2 gap-5 border-y border-border py-5">
            <div className="min-w-0">
              <dt className="text-sm text-muted-foreground">{t("signedInAs")}</dt>
              <dd className="mt-1 font-semibold wrap-anywhere whitespace-pre-wrap">{profile.display_name}</dd>
            </div>
            <div className="min-w-0">
              <dt className="text-sm text-muted-foreground">{t("username")}</dt>
              <dd className="mt-1 font-semibold wrap-anywhere whitespace-pre-wrap">{profile.username}</dd>
            </div>
          </dl>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button asChild className="h-11 text-base">
              <Link href="/workspace/imports">{imports("title")}</Link>
            </Button>
            <Button asChild variant="outline" className="h-11 text-base">
              <Link href="/workspace/reconciliations">{reconciliation("title")}</Link>
            </Button>
            <Button asChild variant="outline" className="h-11 text-base">
              <Link href="/workspace/review">{review("title")}</Link>
            </Button>
            <AccountLinks isAdmin={profile.is_admin} />
          </div>
        </>
      ) : (
        <p className="mt-6 text-destructive" role="alert">{t("sessionUnavailable")}</p>
      )}
    </main>
  );
}
