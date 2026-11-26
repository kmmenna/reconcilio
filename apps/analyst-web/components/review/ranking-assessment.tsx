import { SparklesIcon } from "lucide-react";
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
  const scoreColor = assessment.score < 50 ? "text-destructive" : assessment.score <= 80 ? "text-score-amber" : "text-primary";

  return (
    <section data-ranking-evaluation-id={assessment.evaluation_id} className="mt-4 border-t border-border pt-4">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <Heading className={`flex items-center gap-2 font-semibold ${heading === "h2" ? "text-lg" : "text-sm"}`}>
          <SparklesIcon aria-hidden="true" className="size-4 shrink-0 text-primary" />
          <span>{t(recorded ? "recordedTitle" : "title")}</span>
        </Heading>
        <p className="text-sm font-medium tabular-nums text-muted-foreground">
          {t.rich("summary", {
            rank: assessment.rank,
            score: assessment.score,
            scoreValue: (text) => <span className={`font-semibold ${scoreColor}`}>{text}</span>,
          })}
        </p>
      </div>
      <p className="mt-1.5 text-sm leading-relaxed wrap-anywhere">{assessment.explanations[locale]}</p>
    </section>
  );
}
