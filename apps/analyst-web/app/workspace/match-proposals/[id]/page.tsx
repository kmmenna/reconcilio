import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { LocalTimestamp } from "@/components/local-timestamp";
import { ProposalActions, ReviewDecisions } from "@/components/review/proposal-actions";
import { ProposalEvidence } from "@/components/review/proposal-evidence";
import { RankingAssessment } from "@/components/review/ranking-assessment";
import { TransactionRecord } from "@/components/review/transaction-record";
import { Button } from "@/components/ui/button";
import type { MatchProposalDetail } from "@/lib/reconciliation";
import { readWorkflowID } from "@/lib/auth";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";

export default async function MatchProposalPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const token = await getWorkspaceToken();
  const [proposal, t, workflow] = await Promise.all([
    readWorkspaceResource<MatchProposalDetail>(`/api/match-proposals/${encodeURIComponent(id)}`, token),
    getTranslations("Review"),
    readWorkflowID(),
  ]);

  return (
    <main className="workspace">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="page-title">{t("proposalTitle")}</h1>
          {proposal && <p className="mt-2 text-muted-foreground wrap-anywhere">{proposal.financial_account.name}</p>}
        </div>
        {proposal && (
          <Button asChild variant="outline" className="h-11">
            <Link href={`/workspace/review/${proposal.internal_transaction.id}`}>{t("backToGroup")}</Link>
          </Button>
        )}
      </div>
      {proposal ? (
        <ReviewDecisions workflow={workflow}>
          <section aria-labelledby="proposal-status" className="mt-6 border-y border-border py-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 id="proposal-status" className="text-lg font-semibold">{t(`states.${proposal.status}`)}</h2>
                <p className="mt-1 text-sm text-muted-foreground">{t(`outcomes.${proposal.status}`)}</p>
              </div>
              {proposal.reconciliation_id && (
                <Button asChild variant="outline" className="h-11">
                  <Link href={`/workspace/reconciliations/${proposal.reconciliation_id}`}>{t("viewReconciliation")}</Link>
                </Button>
              )}
              {proposal.status === "pending" && <ProposalActions id={proposal.id} evaluationId={proposal.ranking_assessment?.evaluation_id} />}
            </div>
            <dl className="mt-4 flex flex-wrap gap-x-7 gap-y-3 text-sm">
              <div>
                <dt className="text-muted-foreground">{t("recorded")}</dt>
                <dd className="mt-0.5 tabular-nums">
                  <LocalTimestamp value={proposal.created_at} />
                </dd>
              </div>
              {proposal.resolved_at && (
                <div>
                  <dt className="text-muted-foreground">{t("resolved")}</dt>
                  <dd className="mt-0.5 tabular-nums">
                    <LocalTimestamp value={proposal.resolved_at} />
                  </dd>
                </div>
              )}
              {proposal.actor && (
                <div className="min-w-0">
                  <dt className="text-muted-foreground">{t("actor")}</dt>
                  <dd className="mt-0.5 wrap-anywhere">{proposal.actor.kind === "engine" ? t("engine") : proposal.actor.display_name}</dd>
                </div>
              )}
            </dl>
          </section>

          <div className="mt-6 grid divide-y divide-border rounded-xl border border-border bg-card desktop:grid-cols-2 desktop:divide-x desktop:divide-y-0">
            {[proposal.internal_transaction, proposal.external_transaction].map((item) => (
              <section key={item.id} aria-labelledby={`side-${item.id}`} className="min-w-0 px-5 py-4 desktop:px-6">
                <h2 id={`side-${item.id}`} className="text-sm font-medium text-muted-foreground">{t(item.side)}</h2>
                <p className="mt-1 text-lg font-semibold whitespace-pre-wrap wrap-anywhere">{item.description}</p>
                <TransactionRecord item={item} />
              </section>
            ))}
          </div>
          <section className="mt-7 max-w-xl">
            <ProposalEvidence evidence={proposal.evidence} heading="h2" />
            {proposal.ranking_assessment && (
              <RankingAssessment assessment={proposal.ranking_assessment} heading="h2" recorded={proposal.status !== "pending"} />
            )}
          </section>
        </ReviewDecisions>
      ) : (
        <p className="mt-6 text-destructive" role="alert">{t(proposal === null ? "proposalNotFound" : "unavailable")}</p>
      )}
    </main>
  );
}
