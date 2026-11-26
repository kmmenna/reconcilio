import { getTranslations } from "next-intl/server";

import type { FinancialAccount, SourceEntry } from "@/lib/imports";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";
import { CatalogSection } from "./catalog-section";

export default async function CatalogPage() {
  const token = await getWorkspaceToken();
  const [financialAccounts, sources, t] = await Promise.all([
    readWorkspaceResource<FinancialAccount[]>("/api/financial-accounts", token),
    readWorkspaceResource<SourceEntry[]>("/api/sources", token),
    getTranslations("Catalog"),
  ]);

  return (
    <main className="workspace">
      <h1 className="page-title">{t("title")}</h1>
      <div className="mt-8 grid items-start gap-9 desktop:grid-cols-2">
        <CatalogSection collection="financial-accounts" entries={financialAccounts} />
        <CatalogSection collection="sources" entries={sources} />
      </div>
    </main>
  );
}
