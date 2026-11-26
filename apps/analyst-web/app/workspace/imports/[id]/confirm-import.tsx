"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState, useTransition } from "react";

import { Button } from "@/components/ui/button";
import type { ImportConflict, ImportProblem } from "@/lib/imports";
import type importMessages from "@/messages/imports.en.json";
import { CoverageOverlap } from "../coverage-overlap";
import { ImportConflicts } from "./import-conflicts";

type ProblemCode = keyof typeof importMessages.problems;
type Action = "confirm" | "cancel";

export function ConfirmImport({ id, initialConflicts }: { id: string; initialConflicts: ImportConflict[] }) {
  const t = useTranslations("Imports");
  const router = useRouter();
  const alertRef = useRef<HTMLDivElement>(null);
  const [sending, setSending] = useState<Action | null>(null);
  const [refreshing, startRefresh] = useTransition();
  const [problem, setProblem] = useState<ImportProblem | null>(null);
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
    const fallback = action === "confirm" ? "confirmation_unavailable" : "cancellation_unavailable";
    try {
      const response = await fetch(
        `/api/imports/${encodeURIComponent(id)}/${action}`,
        action === "confirm"
          ? {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ keep_original_conflict_ids: conflicts.map(({ id }) => id) }),
            }
          : { method: "POST" },
      );
      if (response.status === 401) {
        router.replace("/login");
        return;
      }
      if (!response.ok) {
        const error = await response.json() as ImportProblem;
        if (error.context?.preview) setUpdatedConflicts(error.context.preview.conflicts ?? []);
        setProblem(error);
        return;
      }
      startRefresh(() => router.refresh());
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
      {problem && (
        <div ref={alertRef} tabIndex={-1} className="mb-5 rounded-md border border-destructive/35 px-4 py-3 text-destructive" role="alert">
          <p className="font-semibold">{t(t.has(problemKey) ? problemKey : "problems.confirmation_unavailable")}</p>
          {problem.context?.coverage && <CoverageOverlap coverage={problem.context.coverage} />}
        </div>
      )}
      {hasConflicts && (
        <>
          <h2 className="text-lg font-semibold">{t("conflictTitle")}</h2>
          <p className="mt-1 mb-4 max-w-prose text-muted-foreground">{t("conflictIntro")}</p>
          <ImportConflicts conflicts={conflicts} />
        </>
      )}
      <div className={`flex flex-wrap items-center gap-2 ${hasConflicts ? "mt-4" : ""}`}>
        <Button type="button" className="min-h-11 max-w-full whitespace-normal" disabled={pending} onClick={() => resolve("confirm")}>
          {t(sending === "confirm" ? "confirming" : hasConflicts ? "keepOriginal" : "confirm")}
        </Button>
        <Button type="button" variant="outline" className="min-h-11 max-w-full whitespace-normal" disabled={pending} onClick={() => resolve("cancel")}>
          {t(sending === "cancel" ? "canceling" : "cancelImport")}
        </Button>
      </div>
    </section>
  );
}
