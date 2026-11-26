import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { ProposalActions, ReviewDecisions } from "@/components/review/proposal-actions";
import { ProposalEvidence } from "@/components/review/proposal-evidence";
import { RankingAssessment } from "@/components/review/ranking-assessment";
import { TransactionRecord } from "@/components/review/transaction-record";
import { Button } from "@/components/ui/button";
import type { ReviewGroup } from "@/lib/reconciliation";
import { readWorkflowID } from "@/lib/auth";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";

export default async function ReviewGroupPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const token = await getWorkspaceToken();
  const [group, t, workflow] = await Promise.all([
    readWorkspaceResource<ReviewGroup>(`/api/transactions/${encodeURIComponent(id)}/match-proposals`, token),
    getTranslations("Review"),
    readWorkflowID(),
  ]);

  return (
    <main className="workspace">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="page-title">{t("groupTitle")}</h1>
          {group && <p className="mt-2 text-muted-foreground wrap-anywhere">{group.financial_account.name}</p>}
        </div>
        <Button asChild variant="outline" className="h-11">
          <Link href="/workspace/review">{t("backToQueue")}</Link>
        </Button>
      </div>
      {group ? (
        <ReviewDecisions workflow={workflow}>
          <section aria-labelledby="internal-record" className="mt-6 rounded-xl border border-border bg-accent px-5 py-4 desktop:px-6">
            <h2 id="internal-record" className="text-sm font-medium text-muted-foreground">{t("internal")}</h2>
            <p className="mt-1 text-lg font-semibold whitespace-pre-wrap wrap-anywhere">{group.internal_transaction.description}</p>
            <div className="max-w-2xl"><TransactionRecord item={group.internal_transaction} /></div>
          </section>

          {group.proposals.length === 0 ? (
            <p className="mt-6 text-muted-foreground">{t("groupEmpty")}</p>
          ) : (
            <section aria-labelledby="pending-alternatives" className="mt-7">
              <h2 id="pending-alternatives" className="text-lg font-semibold">{t("alternatives", { count: group.proposals.length })}</h2>
              <div className="mt-3 grid gap-4 desktop:grid-cols-2">
                {group.proposals.map((proposal) => {
                  const item = proposal.external_transaction;
                  return (
                    <article key={proposal.id} aria-labelledby={`external-${proposal.id}`} className="flex min-w-0 flex-col rounded-xl border border-border bg-card px-5 py-4">
                      <h3 id={`external-${proposal.id}`} className="text-lg font-semibold whitespace-pre-wrap wrap-anywhere">{item.description || t("alternative")}</h3>
                      <TransactionRecord item={item} />
                      {proposal.ranking_assessment && <RankingAssessment assessment={proposal.ranking_assessment} heading="h4" />}
                      <div className="mt-4 border-t border-border pt-4">
                        <ProposalEvidence evidence={proposal.evidence} heading="h4" />
                      </div>
                      <div className="mt-auto pt-5">
                        <ProposalActions id={proposal.id} evaluationId={proposal.ranking_assessment?.evaluation_id} />
                        <Link className="mt-3 inline-flex min-h-11 items-center text-sm font-medium text-primary underline underline-offset-4 hover:text-primary-hover" href={`/workspace/match-proposals/${proposal.id}`}>
                          {t("viewProposal")}
                        </Link>
                      </div>
                    </article>
                  );
                })}
              </div>
            </section>
          )}
        </ReviewDecisions>
      ) : (
        <p className="mt-6 text-destructive" role="alert">{t(group === null ? "groupNotFound" : "unavailable")}</p>
      )}
    </main>
  );
}
