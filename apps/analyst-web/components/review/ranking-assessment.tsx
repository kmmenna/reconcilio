import { getLocale, getTranslations } from "next-intl/server";

import type { RankingAssessment as Assessment } from "@/lib/reconciliation";

export async function RankingAssessment({
  assessment,
  heading,
  recorded = false,
}: {
  assessment: Assessment;
  heading: "h2" | "h4";
  recorded?: boolean;
}) {
  const [t, locale] = await Promise.all([getTranslations("Review.ranking"), getLocale()]);
  const Heading = heading;

  return (
    <section data-ranking-evaluation-id={assessment.evaluation_id} className="mt-4 border-t border-border pt-4">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <Heading className={heading === "h2" ? "text-lg font-semibold" : "text-sm font-semibold"}>
          {t(recorded ? "recordedTitle" : "title")}
        </Heading>
        <p className="text-sm font-medium tabular-nums text-muted-foreground">
          {t("summary", { rank: assessment.rank, score: assessment.score })}
        </p>
      </div>
      <p className="mt-1.5 text-sm leading-relaxed wrap-anywhere">{assessment.explanations[locale]}</p>
    </section>
  );
}
