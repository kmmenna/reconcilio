import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { SourceSideLabel } from "@/components/source-side-label";
import type { OpenTransaction } from "@/lib/reconciliation";
import { formatMoney, formatTransactionDate } from "@/lib/transaction-format";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";

export default async function OpenTransactionsPage() {
  const token = await getWorkspaceToken();
  const [transactions, t, imports, locale] = await Promise.all([
    readWorkspaceResource<OpenTransaction[]>("/api/transactions?status=open", token),
    getTranslations("Transactions"),
    getTranslations("Imports"),
    getLocale(),
  ]);

  return (
    <main className="workspace">
      <h1 className="page-title">{t("title")}</h1>
      {!transactions ? (
        <p className="mt-6 text-destructive" role="alert">{t("unavailable")}</p>
      ) : transactions.length === 0 ? (
        <p className="mt-7 rounded-xl border border-border bg-card p-5 text-muted-foreground">{t("empty")}</p>
      ) : (
        <div className="mt-7 overflow-hidden rounded-xl border border-border bg-card">
          <table aria-label={t("title")} className="block w-full text-left text-sm lg:table" role="table">
            <thead className="sr-only bg-accent lg:not-sr-only" role="rowgroup">
              <tr role="row">
                <th className="px-5 py-3" scope="col" role="columnheader">{t("transaction")}</th>
                <th className="px-5 py-3" scope="col" role="columnheader">{imports("fields.source_side")}</th>
                <th className="px-5 py-3 text-right" scope="col" role="columnheader">{imports("fields.amount")}</th>
                <th className="w-[29%] px-5 py-3" scope="col" role="columnheader">{t("status")}</th>
              </tr>
            </thead>
            <tbody className="block lg:table-row-group" role="rowgroup">
              {transactions.map((item) => {
                const discrepancy = item.amount_discrepancy;
                return (
                  <tr
                    className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] items-start gap-x-3 gap-y-2 border-t border-border p-4 first:border-t-0 lg:table-row lg:first:border-t"
                    key={item.id}
                    role="row"
                  >
                    <td className="col-span-2 min-w-0 lg:max-w-96 lg:px-5 lg:py-4" role="cell">
                      <p className="text-base font-semibold wrap-anywhere whitespace-pre-wrap">{item.description}</p>
                      <p className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-muted-foreground">
                        {item.source_reference && <span className="wrap-anywhere">{item.source_reference}</span>}
                        <span className="wrap-anywhere">{item.financial_account.name}</span>
                        <time className="tabular-nums" dateTime={item.transaction_date}>{formatTransactionDate(item.transaction_date, locale)}</time>
                      </p>
                      <p className="mt-0.5 text-muted-foreground wrap-anywhere">{item.source.name}</p>
                    </td>
                    <td className="min-w-0 font-medium lg:px-5 lg:py-4 lg:align-top" role="cell">
                      <SourceSideLabel side={item.side}>{t(`sides.${item.side}`)}</SourceSideLabel>
                    </td>
                    <td className="min-w-0 text-right font-semibold tabular-nums wrap-anywhere lg:px-5 lg:py-4 lg:align-top" role="cell">
                      {formatMoney(item.amount, item.currency, locale)}
                    </td>
                    <td className="col-span-2 min-w-0 lg:px-5 lg:py-4 lg:align-top" role="cell">
                      {discrepancy && (
                        <dl className="mb-2 wrap-anywhere">
                          <div className="flex flex-wrap items-baseline gap-x-2">
                            <dt className="font-medium">{t("amountDifference")}</dt>
                            <dd className="font-semibold text-destructive tabular-nums">{formatMoney(discrepancy.amount, discrepancy.currency, locale)}</dd>
                          </div>
                          <div className="flex flex-wrap gap-x-2 text-muted-foreground">
                            <dt>{t("sharedReference")}</dt>
                            <dd className="font-bold text-foreground">{discrepancy.payment_reference}</dd>
                          </div>
                        </dl>
                      )}
                      {item.has_pending_match_proposals ? (
                        item.side === "internal" ? (
                          <Link
                            className="inline-flex min-h-11 items-center font-semibold text-primary underline underline-offset-4 hover:text-primary-hover"
                            href={`/workspace/review/${item.id}`}
                          >
                            {t("pendingProposals")}
                          </Link>
                        ) : (
                          <p className="font-medium">{t("pendingProposals")}</p>
                        )
                      ) : (
                        <p className="text-muted-foreground">{t("noPendingProposals")}</p>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </main>
  );
}
