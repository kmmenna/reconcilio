import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { CreateUserLink } from "@/components/accounts/create-user-link";
import { Button } from "@/components/ui/button";
import { getWorkspaceProfile } from "@/lib/workspace";

export default async function WorkspacePage() {
  const [profile, t, imports, reconciliation, review, transactions, catalog] = await Promise.all([
    getWorkspaceProfile(),
    getTranslations("Workspace"),
    getTranslations("Imports"),
    getTranslations("Reconciliation"),
    getTranslations("Review"),
    getTranslations("Transactions"),
    getTranslations("Catalog"),
  ]);

  return (
    <main className="workspace">
      <h1 className="page-title">{profile ? t("title") : t("unavailable")}</h1>
      {profile ? (
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
          <Button asChild variant="outline" className="h-11 text-base">
            <Link href="/workspace/transactions">{transactions("title")}</Link>
          </Button>
          <Button asChild variant="outline" className="h-11 text-base">
            <Link href="/workspace/catalog">{catalog("title")}</Link>
          </Button>
          {profile.is_admin && <CreateUserLink />}
        </div>
      ) : (
        <p className="mt-6 text-destructive" role="alert">{t("sessionUnavailable")}</p>
      )}
    </main>
  );
}
