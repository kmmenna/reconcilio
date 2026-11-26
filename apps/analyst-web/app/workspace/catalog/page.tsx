import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";
import type { FinancialAccount, SourceEntry } from "@/lib/imports";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";
import { CatalogSection } from "./catalog-section";

export default async function CatalogPage() {
  const token = await getWorkspaceToken();
  const [financialAccounts, sources, t, imports] = await Promise.all([
    readWorkspaceResource<FinancialAccount[]>("/api/financial-accounts", token),
    readWorkspaceResource<SourceEntry[]>("/api/sources", token),
    getTranslations("Catalog"),
    getTranslations("Imports"),
  ]);

  return (
    <main className="workspace">
      <Link href="/workspace" className="mb-4 inline-block text-sm font-medium text-primary underline-offset-4 hover:underline">
        {imports("workspace")}
      </Link>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="page-title">{t("title")}</h1>
        <Button asChild variant="outline" className="h-11">
          <Link href="/workspace/imports">{imports("title")}</Link>
        </Button>
      </div>
      <div className="mt-8 grid items-start gap-9 desktop:grid-cols-2">
        <CatalogSection collection="financial-accounts" entries={financialAccounts} />
        <CatalogSection collection="sources" entries={sources} />
      </div>
    </main>
  );
}
