import { getTranslations } from "next-intl/server";

import { readWorkflowID } from "@/lib/auth";
import type { FinancialAccount } from "@/lib/imports";
import type { ReconciliationJob } from "@/lib/reconciliation";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";
import { JobProgress } from "./job-progress";

export default async function JobPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const token = await getWorkspaceToken();
  const [job, accounts, t, workflow] = await Promise.all([
    /^[1-9]\d*$/.test(id) ? readWorkspaceResource<ReconciliationJob>(`/api/jobs/${id}`, token) : null,
    readWorkspaceResource<FinancialAccount[]>("/api/financial-accounts", token),
    getTranslations("Reconciliation.jobs"),
    readWorkflowID(),
  ]);
  const account = accounts?.find(({ id }) => id === job?.financial_account_id);

  return (
    <main className="workspace">
      <h1 className="page-title">{t("title")}</h1>
      {account && <p className="mt-3 text-muted-foreground wrap-anywhere">{account.name}</p>}
      {job ? <JobProgress key={job.id} initialJob={job} workflow={workflow} /> : (
        <p className="mt-5 text-destructive" role="alert">{t(job === null ? "notFound" : "unavailable")}</p>
      )}
    </main>
  );
}
