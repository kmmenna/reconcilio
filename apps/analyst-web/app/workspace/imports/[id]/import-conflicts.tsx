import { useLocale, useTranslations } from "next-intl";

import type { ConflictField, ImportConflict, TransactionValues } from "@/lib/imports";
import { formatMoney, formatTransactionDate } from "@/lib/transaction-format";

export function ImportConflicts({ conflicts }: { conflicts: ImportConflict[] }) {
  const t = useTranslations("Imports");
  const locale = useLocale();

  function value(record: TransactionValues, field: ConflictField) {
    switch (field) {
      case "amount":
        return formatMoney(record.amount, record.currency, locale);
      case "transaction_date":
        return <time dateTime={record.transaction_date}>{formatTransactionDate(record.transaction_date, locale)}</time>;
      case "payment_reference":
        return record.payment_reference || t("absentReference");
      case "currency":
        return record.currency;
    }
  }

  return (
    <div className="space-y-3">
      {conflicts.map((conflict) => (
        <div className="overflow-hidden rounded-xl border border-border bg-card" key={conflict.id}>
          <table className="w-full table-fixed text-left">
            <caption className="border-b border-border px-4 py-3 text-left">
              <span className="font-semibold wrap-anywhere">{conflict.stored.source_reference}</span>
              <span className="ms-3 text-sm text-muted-foreground">{t("conflictLine", { line: conflict.line })}</span>
            </caption>
            <thead className="bg-accent text-sm">
              <tr>
                <td className="w-[30%]" />
                <th className="px-2 py-2.5 font-semibold wrap-anywhere sm:px-4" scope="col">{t("storedValue")}</th>
                <th className="px-2 py-2.5 font-semibold wrap-anywhere sm:px-4" scope="col">{t("incomingValue")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {conflict.fields.map((field) => (
                <tr className="align-top" key={field}>
                  <th className="px-3 py-3 text-sm font-medium wrap-anywhere sm:px-4" scope="row">{t(`fields.${field}`)}</th>
                  <td className="px-2 py-3 font-semibold tabular-nums wrap-anywhere sm:px-4">{value(conflict.stored, field)}</td>
                  <td className="px-2 py-3 tabular-nums wrap-anywhere sm:px-4">{value(conflict.incoming, field)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}
    </div>
  );
}
