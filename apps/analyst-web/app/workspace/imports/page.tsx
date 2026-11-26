import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { requestAPI } from "@/lib/auth";
import type { FinancialAccount, Source } from "@/lib/imports";
import { getWorkspaceToken } from "@/lib/workspace";
import { ImportForm } from "./import-form";

async function readCatalog(token: string) {
  const init = { headers: { Authorization: `Bearer ${token}` } };
  const [sourcesResponse, accountsResponse] = await Promise.all([
    requestAPI("/api/sources", init),
    requestAPI("/api/financial-accounts", init),
  ]);

  if (sourcesResponse.status === 401 || accountsResponse.status === 401) return null;
  if (!sourcesResponse.ok || !accountsResponse.ok) {
    throw new Error("The Go API could not load the import selections.");
  }

  const [sources, financialAccounts]: [Source[], FinancialAccount[]] = await Promise.all([
    sourcesResponse.json(),
    accountsResponse.json(),
  ]);
  return { sources, financialAccounts };
}

export default async function ImportPage() {
  const token = await getWorkspaceToken();
  const catalog = await readCatalog(token).catch(() => undefined);
  if (catalog === null) redirect("/login");
  const [t, catalogLabels] = await Promise.all([getTranslations("Imports"), getTranslations("Catalog")]);

  return (
    <main className="workspace">
      <h1 className="page-title">{t("title")}</h1>
      {catalog ? (
        catalog.sources.length > 0 && catalog.financialAccounts.length > 0 ? (
          <>
            <p className="mt-3 max-w-prose text-muted-foreground">{t("intro")}</p>
            <ImportForm sources={catalog.sources} financialAccounts={catalog.financialAccounts} />
          </>
        ) : (
          <div className="mt-5">
            <p className="text-muted-foreground">{t("noCatalog")}</p>
            <ul className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-sm font-medium text-primary underline underline-offset-4">
              {catalog.sources.length === 0 && (
                <li><Link href="/workspace/catalog#sources">{catalogLabels("sources")}</Link></li>
              )}
              {catalog.financialAccounts.length === 0 && (
                <li><Link href="/workspace/catalog#financial-accounts">{catalogLabels("accounts")}</Link></li>
              )}
            </ul>
          </div>
        )
      ) : (
        <p className="mt-5 text-destructive" role="alert">{t("problems.catalog_unavailable")}</p>
      )}
    </main>
  );
}
