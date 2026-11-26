import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";
import type { ReviewQueueEntry } from "@/lib/reconciliation";
import { formatMoney, formatTransactionDate } from "@/lib/transaction-format";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";

export default async function ReviewQueuePage() {
  const token = await getWorkspaceToken();
  const [groups, t, locale] = await Promise.all([
    readWorkspaceResource<ReviewQueueEntry[]>("/api/review-queue", token),
    getTranslations("Review"),
    getLocale(),
  ]);

  return (
    <main className="workspace">
      <h1 className="page-title">{t("title")}</h1>
      {!groups ? (
        <p className="mt-6 text-destructive" role="alert">{t("unavailable")}</p>
      ) : groups.length === 0 ? (
        <p className="mt-7 rounded-xl border border-border bg-card p-5 text-muted-foreground">{t("empty")}</p>
      ) : (
        <ul className="mt-7 divide-y divide-border rounded-xl border border-border bg-card">
          {groups.map(({ internal_transaction: item, financial_account: account, pending_count }) => (
            <li className="flex flex-wrap items-center justify-between gap-4 px-5 py-4 desktop:px-6" key={item.id}>
              <div className="min-w-0 flex-1 basis-64">
                <p className="font-semibold whitespace-pre-wrap wrap-anywhere">{item.description}</p>
                <p className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-sm text-muted-foreground">
                  {item.source_reference && <span className="wrap-anywhere">{item.source_reference}</span>}
                  <span className="wrap-anywhere">{account.name}</span>
                  <time dateTime={item.transaction_date} className="tabular-nums">
                    {formatTransactionDate(item.transaction_date, locale)}
                  </time>
                </p>
              </div>
              <div className="min-w-0">
                <p className="font-semibold tabular-nums wrap-anywhere">{formatMoney(item.amount, item.currency, locale)}</p>
                <p className="text-sm text-muted-foreground">{t("alternatives", { count: pending_count })}</p>
              </div>
              <Button asChild variant="outline" className="h-11">
                <Link href={`/workspace/review/${item.id}`}>{t("compare")}</Link>
              </Button>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
