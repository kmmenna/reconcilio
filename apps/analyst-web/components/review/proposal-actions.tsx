"use client";

import { correlatedFetch } from "@/lib/correlation";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { createContext, type ReactNode, useContext, useRef, useState, useTransition } from "react";

import { Button } from "@/components/ui/button";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

type Decision = "confirm" | "reject";

const DecisionContext = createContext<{
  decide: (id: string, action: Decision, evaluationId?: string) => void;
  submitting: string | null;
  navigating: boolean;
} | null>(null);

export function ReviewDecisions({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const active = useRef(false);
  const [submitting, setSubmitting] = useState<string | null>(null);
  const [navigating, startNavigation] = useTransition();
  const [errorCode, setErrorCode] = useState<string | null>(null);
  const error = useProblemMessage(errorCode, "review_unavailable");

  async function decide(id: string, action: Decision, evaluationId?: string) {
    if (active.current || navigating) return;
    active.current = true;
    setSubmitting(id);
    setErrorCode(null);
    const detailPath = `/workspace/match-proposals/${id}`;

    try {
      const response = await correlatedFetch(`/api/match-proposals/${id}/${action}`, {
        method: "POST",
        ...(evaluationId && {
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ ranking_evaluation_id: evaluationId }),
        }),
      });
      if (response.ok) {
        startNavigation(() => {
          if (pathname !== detailPath) router.push(detailPath);
          router.refresh();
        });
      } else if (response.status === 401) {
        router.replace("/login");
      } else if (response.status === 409) {
        setErrorCode("match_proposal_conflict");
        startNavigation(() => router.refresh());
      } else {
        setErrorCode((await readProblemCode(response)) ?? "review_unavailable");
      }
    } catch {
      setErrorCode("review_unavailable");
    } finally {
      active.current = false;
      setSubmitting(null);
    }
  }

  return (
    <DecisionContext.Provider value={{ decide, submitting, navigating }}>
      {error && <p className="mt-5 rounded-xl border border-destructive bg-card px-4 py-3 text-sm font-medium text-destructive" role="alert">{error}</p>}
      {children}
    </DecisionContext.Provider>
  );
}

export function ProposalActions({ id, evaluationId }: { id: string; evaluationId?: string }) {
  const t = useTranslations("Review");
  const state = useContext(DecisionContext);
  if (!state) throw new Error("ProposalActions must be rendered inside ReviewDecisions.");
  const busy = state.submitting !== null || state.navigating;

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button type="button" className="min-h-11" disabled={busy} onClick={() => state.decide(id, "confirm", evaluationId)}>
        {t("confirm")}
      </Button>
      <Button type="button" variant="outline" className="min-h-11" disabled={busy} onClick={() => state.decide(id, "reject", evaluationId)}>
        {t("reject")}
      </Button>
      {state.submitting === id && <span className="text-sm text-muted-foreground" role="status">{t("saving")}</span>}
    </div>
  );
}
