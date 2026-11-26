import { getLocale, getTranslations } from "next-intl/server";

import type { ProposalEvidence as MatchEvidence } from "@/lib/reconciliation";
import { formatMoney } from "@/lib/transaction-format";

export async function ProposalEvidence({ evidence, heading }: { evidence: MatchEvidence; heading: "h2" | "h4" }) {
  const [t, locale] = await Promise.all([getTranslations("Review.evidence"), getLocale()]);
  const Heading = heading;

  return (
    <>
      <Heading className={heading === "h2" ? "text-lg font-semibold" : "text-sm font-semibold"}>
        {t("title")}
      </Heading>
      <dl className="mt-3 grid grid-cols-2 gap-3 text-sm">
        <div className="col-span-2">
          <dt className="text-muted-foreground">{t("reference")}</dt>
          <dd className="mt-0.5 font-medium">{t(`relations.${evidence.reference_relation}`)}</dd>
          {evidence.internal_reference_count !== undefined && evidence.external_reference_count !== undefined && (
            <dd className="mt-0.5 text-muted-foreground">
              {t("referenceCounts", { internal: evidence.internal_reference_count, external: evidence.external_reference_count })}
            </dd>
          )}
        </div>
        <div className="min-w-0">
          <dt className="text-muted-foreground">{t("dateDifference")}</dt>
          <dd className="mt-0.5 font-medium tabular-nums">{t("daysApart", { count: evidence.days_apart })}</dd>
        </div>
        <div className="min-w-0">
          <dt className="text-muted-foreground">{t("matchedAmount")}</dt>
          <dd className="mt-0.5 font-medium tabular-nums wrap-anywhere">{formatMoney(evidence.amount, evidence.currency, locale)}</dd>
        </div>
      </dl>
    </>
  );
}
