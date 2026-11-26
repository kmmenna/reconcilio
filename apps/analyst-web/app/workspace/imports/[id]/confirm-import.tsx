"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState, useTransition } from "react";

import { Button } from "@/components/ui/button";
import { readProblemCode } from "@/lib/problem-message";
import type importMessages from "@/messages/imports.en.json";

type ConfirmationProblem = keyof typeof importMessages.problems;

export function ConfirmImport({ id }: { id: string }) {
  const t = useTranslations("Imports");
  const router = useRouter();
  const [sending, setSending] = useState(false);
  const [refreshing, startRefresh] = useTransition();
  const [problem, setProblem] = useState<ConfirmationProblem | null>(null);
  const pending = sending || refreshing;

  async function confirm() {
    if (pending) return;
    setSending(true);
    setProblem(null);
    try {
      const response = await fetch(`/api/imports/${encodeURIComponent(id)}/confirm`, { method: "POST" });
      if (response.status === 401) {
        router.replace("/login");
        return;
      }
      if (!response.ok) {
        const code = await readProblemCode(response) as ConfirmationProblem | undefined;
        setProblem(code && t.has(`problems.${code}`) ? code : "confirmation_unavailable");
        return;
      }
      startRefresh(() => router.refresh());
    } catch {
      setProblem("confirmation_unavailable");
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="min-w-0">
      <Button type="button" className="h-11" disabled={pending} onClick={confirm}>
        {t(pending ? "confirming" : "confirm")}
      </Button>
      {problem && <p className="mt-2 max-w-xs text-sm text-destructive" role="alert">{t(`problems.${problem}`)}</p>}
    </div>
  );
}
