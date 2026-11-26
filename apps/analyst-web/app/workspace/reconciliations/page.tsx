import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { LocalTimestamp } from "@/components/local-timestamp";
import type { FinancialAccount } from "@/lib/imports";
import type { Reconciliation } from "@/lib/reconciliation";
import { formatMoney } from "@/lib/transaction-format";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";
import { AccountFilter } from "./account-filter";

type Props = { searchParams: Promise<{ financial_account_id?: string | string[] }> };

export default async function ReconciliationsPage({ searchParams }: Props) {
  const token = await getWorkspaceToken();
  const { financial_account_id: value } = await searchParams;
  const account = Array.isArray(value) ? value[0] : value;
  const query = account ? `?financial_account_id=${encodeURIComponent(account)}` : "";
  const [accounts, results, t, locale] = await Promise.all([
    readWorkspaceResource<FinancialAccount[]>("/api/financial-accounts", token),
    readWorkspaceResource<Reconciliation[]>(`/api/reconciliations${query}`, token),
    getTranslations("Reconciliation"),
    getLocale(),
  ]);

  const selectedAccount = accounts?.find(({ id }) => id === account)?.id ?? "";

  return (
    <main className="workspace">
      <h1 className="page-title">{t("title")}</h1>
      {accounts && <AccountFilter key={selectedAccount} accounts={accounts} initialAccount={selectedAccount} />}
      {!results || !accounts ? (
        <p className="mt-6 text-destructive" role="alert">{t("unavailable")}</p>
      ) : results.length === 0 ? (
        <p className="mt-7 rounded-xl border border-border bg-card p-5 text-muted-foreground">{t("empty")}</p>
      ) : (
        <div className="mt-7 overflow-hidden rounded-xl border border-border bg-card">
          <table aria-label={t("title")} className="block w-full text-left text-sm lg:table" role="table">
            <thead className="sr-only bg-accent lg:not-sr-only" role="rowgroup">
              <tr role="row">
                <th scope="col" role="columnheader" className="px-5 py-3">{t("filter")}</th>
                <th scope="col" role="columnheader" className="px-5 py-3 text-right">{t("amount")}</th>
                <th scope="col" role="columnheader" className="px-5 py-3">{t("completedAt")}</th>
                <th scope="col" role="columnheader" className="px-5 py-3">{t("method")}</th>
                <th scope="col" role="columnheader" className="sr-only">{t("openDetail")}</th>
              </tr>
            </thead>
            <tbody className="block lg:table-row-group" role="rowgroup">
              {results.map((result) => (
                <tr className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 items-start gap-y-2 border-t border-border p-4 first:border-t-0 lg:table-row lg:first:border-t" key={result.id} role="row">
                  <td className="min-w-0 font-medium wrap-anywhere lg:max-w-72 lg:px-5 lg:py-4" role="cell">{result.financial_account.name}</td>
                  <td className="text-right font-semibold tabular-nums lg:px-5 lg:py-4 lg:whitespace-nowrap" role="cell">{formatMoney(result.amount, result.currency, locale)}</td>
                  <td className="col-span-2 text-muted-foreground tabular-nums lg:px-5 lg:py-4" role="cell">
                    <LocalTimestamp value={result.confirmed_at} />
                  </td>
                  <td className="col-span-2 lg:px-5 lg:py-4" role="cell">{t(`methods.${result.method}`)}</td>
                  <td className="col-span-2 lg:px-5 lg:text-right lg:whitespace-nowrap" role="cell">
                    <Link
                      className="inline-flex min-h-11 items-center font-medium text-primary underline underline-offset-4 hover:text-primary-hover"
                      href={`/workspace/reconciliations/${result.id}`}
                    >
                      {t("openDetail")}
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  );
}
