import { useLocale, useTranslations } from "next-intl";

import type { CoverageOverlap as Overlap } from "@/lib/imports";
import { formatTransactionDate } from "@/lib/transaction-format";

export function CoverageOverlap({ coverage }: { coverage: Overlap }) {
  const t = useTranslations("Imports");
  const locale = useLocale();
  return (
    <p className="mt-2">
      {t("coverageDates", {
        account: coverage.financial_account.name,
        start: formatTransactionDate(coverage.period_start, locale),
        end: formatTransactionDate(coverage.period_end, locale),
      })}
    </p>
  );
}
