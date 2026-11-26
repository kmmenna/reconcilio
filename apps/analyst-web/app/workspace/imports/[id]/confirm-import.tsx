"use client";

import { correlatedFetch, workflowURL } from "@/lib/correlation";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState, useTransition } from "react";

import { SourceSideLabel } from "@/components/source-side-label";
import { Button } from "@/components/ui/button";
import type { FinancialAccount, ImportConflict, ImportProblem, ImportView, Source } from "@/lib/imports";
import type importMessages from "@/messages/imports.en.json";
import { CoverageOverlap } from "../coverage-overlap";
import { ImportIssues } from "../import-issues";
import { ImportConflicts } from "./import-conflicts";

type ProblemCode = keyof typeof importMessages.problems;
type Action = "confirm" | "cancel" | "refresh-preview";

export function ConfirmImport({ id, workflow, initialConflicts, accounts }: {
  id: string;
  workflow: string;
  initialConflicts: ImportConflict[];
  accounts: FinancialAccount[];
}) {
  const t = useTranslations("Imports");
  const router = useRouter();
  const alertRef = useRef<HTMLDivElement>(null);
  const [sending, setSending] = useState<Action | null>(null);
  const [refreshing, startRefresh] = useTransition();
  const [problem, setProblem] = useState<ImportProblem | null>(null);
  const [sideChange, setSideChange] = useState<{ preview: Source["side"]; current: Source["side"] } | null>(null);
  const [updatedConflicts, setUpdatedConflicts] = useState<ImportConflict[] | null>(null);
  const conflicts = updatedConflicts ?? initialConflicts;
  const pending = sending !== null || refreshing;

  useEffect(() => {
    if (problem) alertRef.current?.focus();
  }, [problem]);

  async function resolve(action: Action) {
    if (pending) return;
    setSending(action);
    setProblem(null);
    const fallback = {
      confirm: "confirmation_unavailable",
      cancel: "cancellation_unavailable",
      "refresh-preview": "refresh_unavailable",
    }[action];
    try {
      const response = await correlatedFetch(
        `/api/imports/${encodeURIComponent(id)}/${action}`,
        action === "confirm"
          ? {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ keep_original_conflict_ids: conflicts.map(({ id }) => id) }),
            }
          : { method: "POST" },
        workflow,
      );
      if (response.status === 401) {
        router.replace("/login");
        return;
      }
      if (!response.ok) {
        const error = await response.json() as ImportProblem;
        if (error.context?.preview) setUpdatedConflicts(error.context.preview.conflicts ?? []);
        const previous = error.context?.preview_source_side;
        const current = error.context?.current_source_side;
        if (previous && current) setSideChange({ preview: previous, current });
        setProblem(error);
        return;
      }
      if (action === "refresh-preview") {
        const preview = await response.json() as ImportView;
        startRefresh(() => router.push(workflowURL(`/workspace/imports/${encodeURIComponent(preview.id)}`, workflow)));
      } else {
        startRefresh(() => router.refresh());
      }
    } catch {
      setProblem({ code: fallback });
    } finally {
      setSending(null);
    }
  }

  const problemKey = `problems.${problem?.code as ProblemCode}` as const;
  const hasConflicts = conflicts.length > 0;
  return (
    <section className="mt-8" aria-label={t("reviewTitle")} aria-busy={pending}>
      {(problem || sideChange) && (
        <div ref={alertRef} tabIndex={-1} className="mb-5 rounded-md border border-destructive/35 px-4 py-3 text-destructive" role="alert">
          <p className="font-semibold">{t(sideChange ? "problems.source_side_changed" : t.has(problemKey) ? problemKey : "problems.confirmation_unavailable")}</p>
          {sideChange && (
            <dl className="mt-2 flex flex-wrap gap-x-6 gap-y-2 text-sm">
              <div>
                <dt>{t("previousSide")}</dt>
                <dd className="font-semibold"><SourceSideLabel side={sideChange.preview}>{t(`sides.${sideChange.preview}`)}</SourceSideLabel></dd>
              </div>
              <div>
                <dt>{t("currentSide")}</dt>
                <dd className="font-semibold"><SourceSideLabel side={sideChange.current}>{t(`sides.${sideChange.current}`)}</SourceSideLabel></dd>
              </div>
            </dl>
          )}
          {sideChange && problem && problem.code !== "source_side_changed" && (
            <p className="mt-3 font-semibold">{t(t.has(problemKey) ? problemKey : "problems.refresh_unavailable")}</p>
          )}
          {problem?.context?.coverage && <CoverageOverlap coverage={problem.context.coverage} />}
          <ImportIssues issues={problem?.issues} />
        </div>
      )}
      {hasConflicts && (
        <>
          <h2 className="text-lg font-semibold">{t("conflictTitle")}</h2>
          <p className="mt-1 mb-4 max-w-prose text-muted-foreground">{t("conflictIntro")}</p>
          <ImportConflicts accounts={accounts} conflicts={conflicts} />
        </>
      )}
      <div className={`flex flex-wrap items-center gap-2 ${hasConflicts ? "mt-4" : ""}`}>
        {sideChange ? (
          <Button type="button" className="min-h-11 max-w-full whitespace-normal" disabled={pending} onClick={() => resolve("refresh-preview")}>
            {t(sending === "refresh-preview" ? "refreshingPreview" : "refreshPreview")}
          </Button>
        ) : (
          <Button type="button" className="min-h-11 max-w-full whitespace-normal" disabled={pending} onClick={() => resolve("confirm")}>
            {t(sending === "confirm" ? "confirming" : hasConflicts ? "keepOriginal" : "confirm")}
          </Button>
        )}
        <Button type="button" variant="outline" className="min-h-11 max-w-full whitespace-normal" disabled={pending} onClick={() => resolve("cancel")}>
          {t(sending === "cancel" ? "canceling" : "cancelImport")}
        </Button>
      </div>
    </section>
  );
}
