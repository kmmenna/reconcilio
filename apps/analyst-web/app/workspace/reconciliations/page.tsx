import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import type { FinancialAccount } from "@/lib/imports";
import type { Reconciliation } from "@/lib/reconciliation";
import { formatMoney, formatTimestamp } from "@/lib/transaction-format";
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
        <p className="mt-7 border-y border-border py-6 text-muted-foreground">{t("empty")}</p>
      ) : (
        <table aria-label={t("title")} className="mt-7 block w-full border border-border bg-card text-left text-sm desktop:table" role="table">
          <thead className="sr-only bg-accent desktop:not-sr-only" role="rowgroup">
            <tr role="row">
              <th scope="col" role="columnheader" className="px-4 py-3">{t("filter")}</th>
              <th scope="col" role="columnheader" className="px-4 py-3 text-right">{t("amount")}</th>
              <th scope="col" role="columnheader" className="px-4 py-3">{t("completedAt")}</th>
              <th scope="col" role="columnheader" className="px-4 py-3">{t("method")}</th>
              <th scope="col" role="columnheader" className="sr-only">{t("openDetail")}</th>
            </tr>
          </thead>
          <tbody className="block desktop:table-row-group" role="rowgroup">
            {results.map((result) => (
              <tr className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-1 border-t border-border px-4 py-3 first:border-t-0 desktop:table-row desktop:first:border-t" key={result.id} role="row">
                <td className="min-w-0 font-medium wrap-anywhere desktop:max-w-72 desktop:px-4 desktop:py-3" role="cell">{result.financial_account.name}</td>
                <td className="text-right font-semibold tabular-nums desktop:px-4 desktop:py-3 desktop:whitespace-nowrap" role="cell">{formatMoney(result.amount, result.currency, locale)}</td>
                <td className="col-span-2 text-muted-foreground tabular-nums desktop:px-4 desktop:py-3" role="cell">
                  <time dateTime={result.confirmed_at}>{formatTimestamp(result.confirmed_at, locale)}</time>
                </td>
                <td className="col-span-2 mt-1 desktop:mt-0 desktop:px-4 desktop:py-3" role="cell">{t(`methods.${result.method}`)}</td>
                <td className="col-span-2 desktop:px-4 desktop:text-right desktop:whitespace-nowrap" role="cell">
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
      )}
    </main>
  );
}
