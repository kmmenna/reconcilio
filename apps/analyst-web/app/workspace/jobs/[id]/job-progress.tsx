"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import type { ReconciliationJob } from "@/lib/reconciliation";
import { formatTimestamp } from "@/lib/transaction-format";

function isFinal(status: ReconciliationJob["status"]) {
  return status === "completed" || status === "failed" || status === "cancelled";
}

export function JobProgress({ initialJob }: { initialJob: ReconciliationJob }) {
  const t = useTranslations("Reconciliation");
  const locale = useLocale();
  const router = useRouter();
  const [job, setJob] = useState(initialJob);
  const [problem, setProblem] = useState<"pollUnavailable" | "notFound" | null>(null);
  const { id, status } = job;
  const finished = isFinal(status);
  const hasPartialResults = status === "failed" && ((job.automatic_count ?? 0) > 0 || (job.proposal_count ?? 0) > 0);

  useEffect(() => {
    if (finished) return;
    const abort = new AbortController();
    let timer: ReturnType<typeof setTimeout>;

    async function poll() {
      try {
        const response = await fetch(`/api/jobs/${id}`, { signal: abort.signal });
        if (response.status === 401) {
          router.replace("/login");
          return;
        }
        if (response.status === 404) {
          setProblem("notFound");
          return;
        }
        if (!response.ok) throw new Error("The job status could not be read.");
        const latest = await response.json() as ReconciliationJob;
        setJob(latest);
        setProblem(null);
        if (isFinal(latest.status)) return;
      } catch {
        if (abort.signal.aborted) return;
        setProblem("pollUnavailable");
      }
      timer = setTimeout(poll, 1200);
    }

    timer = setTimeout(poll, 1200);
    return () => {
      abort.abort();
      clearTimeout(timer);
    };
  }, [finished, id, router]);

  return (
    <>
      <div className="mt-7 max-w-2xl rounded-xl border border-border bg-card px-5 py-5 desktop:px-6">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <p className={`text-lg font-semibold ${status === "failed" ? "text-destructive" : "text-primary"}`} role="status">
            {t(`jobs.status.${status}`)}
          </p>
          {job.attempt > 0 && !finished && <p className="text-sm text-muted-foreground">{t("jobs.attempt", { attempt: job.attempt, max: job.max_attempts })}</p>}
        </div>
        {job.completed_at && (
          <dl className="mt-4 border-t border-border pt-4">
            <dt className="text-sm text-muted-foreground">{t("jobs.completedAt")}</dt>
            <dd className="mt-1 font-medium tabular-nums">
              <time dateTime={job.completed_at}>{formatTimestamp(job.completed_at, locale)}</time>
            </dd>
          </dl>
        )}
        {hasPartialResults && <p className="mt-4 font-medium">{t("jobs.partialResults")}</p>}
        {job.automatic_count !== undefined && job.proposal_count !== undefined && (
          <ul className="mt-3 space-y-1 text-sm text-muted-foreground">
            <li>{t("jobs.result", { count: job.automatic_count })}</li>
            <li>{t("jobs.proposals", { count: job.proposal_count })}</li>
          </ul>
        )}
      </div>
      {problem && <p className="mt-3 text-sm text-destructive" role="alert">{t(`jobs.${problem}`)}</p>}
      <Button asChild variant="outline" className="mt-5 h-11">
        <Link href={`/workspace/reconciliations?financial_account_id=${encodeURIComponent(job.financial_account_id)}`}>
          {t("backToList")}
        </Link>
      </Button>
    </>
  );
}
