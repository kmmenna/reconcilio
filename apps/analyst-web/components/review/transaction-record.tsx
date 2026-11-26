import { getLocale, getTranslations } from "next-intl/server";

import type { Transaction } from "@/lib/reconciliation";
import { formatMoney, formatTransactionDate } from "@/lib/transaction-format";

export async function TransactionRecord({ item }: { item: Transaction }) {
  const [imports, reconciliation, locale] = await Promise.all([
    getTranslations("Imports"),
    getTranslations("Reconciliation"),
    getLocale(),
  ]);

  return (
    <>
      <p className="mt-1 text-sm text-muted-foreground wrap-anywhere">{item.source.name}</p>
      <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
        <div className="min-w-0">
          <dt className="text-muted-foreground">{reconciliation("amount")}</dt>
          <dd className="mt-0.5 font-semibold tabular-nums wrap-anywhere">{formatMoney(item.amount, item.currency, locale)}</dd>
        </div>
        <div className="min-w-0">
          <dt className="text-muted-foreground">{imports("fields.transaction_date")}</dt>
          <dd className="mt-0.5 tabular-nums">
            <time dateTime={item.transaction_date}>{formatTransactionDate(item.transaction_date, locale)}</time>
          </dd>
        </div>
        <div className="min-w-0">
          <dt className="text-muted-foreground">{imports("fields.source_reference")}</dt>
          <dd className="mt-0.5 wrap-anywhere">{item.source_reference || imports("absentReference")}</dd>
        </div>
        <div className="min-w-0">
          <dt className="text-muted-foreground">{imports("fields.payment_reference")}</dt>
          <dd className="mt-0.5 wrap-anywhere">{item.payment_reference || imports("absentReference")}</dd>
        </div>
      </dl>
    </>
  );
}
