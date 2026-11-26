import { getLocale, getTranslations } from "next-intl/server";

import { SourceSideLabel } from "@/components/source-side-label";
import type { ProposalEvidence as MatchEvidence } from "@/lib/reconciliation";
import { formatMoney } from "@/lib/transaction-format";
import { TransactionDirection } from "./transaction-direction";

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
              {t.rich("referenceCounts", {
                internal: evidence.internal_reference_count,
                external: evidence.external_reference_count,
                internalSide: (text) => <SourceSideLabel side="internal">{text}</SourceSideLabel>,
                externalSide: (text) => <SourceSideLabel side="external">{text}</SourceSideLabel>,
              })}
            </dd>
          )}
        </div>
        <div className="min-w-0">
          <dt className="text-muted-foreground">{t("dateDifference")}</dt>
          <dd className="mt-0.5 font-medium tabular-nums">{t("daysApart", { count: evidence.days_apart })}</dd>
        </div>
        <div className="min-w-0">
          <dt className="text-muted-foreground">{t("matchedAmount")}</dt>
          <dd className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 font-medium tabular-nums wrap-anywhere">
            <span>{formatMoney(evidence.amount, evidence.currency, locale)}</span>
            <TransactionDirection amount={evidence.amount} />
          </dd>
        </div>
      </dl>
    </>
  );
}
