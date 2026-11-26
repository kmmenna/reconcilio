"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState, useTransition } from "react";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import type { FinancialAccount } from "@/lib/imports";

export function AccountFilter({ accounts, initialAccount }: { accounts: FinancialAccount[]; initialAccount: string }) {
  const t = useTranslations("Reconciliation");
  const router = useRouter();
  const [account, setAccount] = useState(initialAccount);
  const [sending, setSending] = useState(false);
  const [navigating, startNavigation] = useTransition();
  const [problem, setProblem] = useState<"unavailable" | "notFound" | null>(null);
  const pending = sending || navigating;

  async function reprocess() {
    if (!account || pending) return;
    setSending(true);
    setProblem(null);
    try {
      const response = await fetch(`/api/financial-accounts/${encodeURIComponent(account)}/reprocess`, { method: "POST" });
      if (response.status === 401) {
        startNavigation(() => router.replace("/login"));
        return;
      }
      if (response.status === 404) {
        setProblem("notFound");
        return;
      }
      if (!response.ok) throw new Error("The reprocessing request failed.");
      const { job_id: jobID } = await response.json() as { job_id: number };
      startNavigation(() => router.push(`/workspace/jobs/${jobID}`));
    } catch {
      setProblem("unavailable");
    } finally {
      setSending(false);
    }
  }

  return (
    <>
      <form action="/workspace/reconciliations" className="mt-7 flex flex-wrap items-end gap-3" method="get" aria-busy={pending}>
        <div className="min-w-0 space-y-2">
          <Label htmlFor="reconciliation-account">{t("filter")}</Label>
          <NativeSelect
            id="reconciliation-account"
            name="financial_account_id"
            className="h-11 max-w-[min(21rem,calc(100vw-2.6rem))] bg-card shadow-none"
            value={account}
            onChange={(event) => {
              setAccount(event.target.value);
              setProblem(null);
            }}
            disabled={pending}
          >
            <NativeSelectOption value="">{t("allAccounts")}</NativeSelectOption>
            {accounts.map(({ id, name }) => <NativeSelectOption key={id} value={id}>{name}</NativeSelectOption>)}
          </NativeSelect>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button type="submit" variant="outline" className="h-11" disabled={pending}>{t("applyFilter")}</Button>
          <Button type="button" variant="outline" className="min-h-11 whitespace-normal" onClick={reprocess} disabled={!account || pending}>
            {t(pending ? "reprocess.queueing" : "reprocess.action")}
          </Button>
        </div>
      </form>
      {problem && <p className="mt-3 text-destructive" role="alert">{t(`reprocess.${problem}`)}</p>}
    </>
  );
}
