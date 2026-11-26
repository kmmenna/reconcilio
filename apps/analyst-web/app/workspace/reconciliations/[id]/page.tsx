import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";
import type { ReconciliationDetail } from "@/lib/reconciliation";
import { formatMoney, formatTimestamp, formatTransactionDate } from "@/lib/transaction-format";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";

export default async function ReconciliationPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const token = await getWorkspaceToken();
  const [result, t, imports, locale] = await Promise.all([
    readWorkspaceResource<ReconciliationDetail>(`/api/reconciliations/${encodeURIComponent(id)}`, token),
    getTranslations("Reconciliation"),
    getTranslations("Imports"),
    getLocale(),
  ]);

  return (
    <main className="workspace">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="page-title">{t("detailTitle")}</h1>
          {result && <p className="mt-3 text-muted-foreground wrap-anywhere">{result.financial_account.name}</p>}
        </div>
        <Button asChild variant="outline" className="h-11">
          <Link href="/workspace/reconciliations">{t("backToList")}</Link>
        </Button>
      </div>
      {result ? (
        <>
          <dl className="mt-7 grid gap-4 border-y border-border py-5 sm:grid-cols-3">
            <div>
              <dt className="text-sm text-muted-foreground">{t("method")}</dt>
              <dd className="mt-1 font-semibold">{t(`methods.${result.method}`)}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">{t("decisionActor")}</dt>
              <dd className="mt-1 font-semibold wrap-anywhere">{result.actor.kind === "engine" ? t("engine") : result.actor.display_name}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">{t("completedAt")}</dt>
              <dd className="mt-1 font-semibold tabular-nums">
                <time dateTime={result.confirmed_at}>{formatTimestamp(result.confirmed_at, locale)}</time>
              </dd>
            </div>
          </dl>

          <div className="mt-7 grid divide-y divide-border rounded-xl border border-border bg-card sm:grid-cols-2 sm:divide-x sm:divide-y-0">
            {result.members.map((member) => (
              <section aria-labelledby={`member-${member.id}`} className="min-w-0 px-5 py-5 desktop:px-6" key={member.id}>
                <h2 className="text-lg font-semibold" id={`member-${member.id}`}>{imports(`sides.${member.side}`)}</h2>
                <p className="mt-1 text-sm text-muted-foreground wrap-anywhere">{member.source.name}</p>
                <p className="mt-4 font-medium whitespace-pre-wrap wrap-anywhere">{member.description}</p>
                <dl className="mt-4 grid grid-cols-2 gap-x-3 gap-y-4 border-t border-border pt-4">
                  <div className="min-w-0">
                    <dt className="text-sm text-muted-foreground">{t("amount")}</dt>
                    <dd className="mt-1 font-semibold tabular-nums wrap-anywhere">{formatMoney(member.amount, member.currency, locale)}</dd>
                  </div>
                  <div className="min-w-0">
                    <dt className="text-sm text-muted-foreground">{imports("fields.transaction_date")}</dt>
                    <dd className="mt-1 tabular-nums">
                      <time dateTime={member.transaction_date}>{formatTransactionDate(member.transaction_date, locale)}</time>
                    </dd>
                  </div>
                  <div className="min-w-0">
                    <dt className="text-sm text-muted-foreground">{imports("fields.source_reference")}</dt>
                    <dd className="mt-1 wrap-anywhere">{member.source_reference || imports("absentReference")}</dd>
                  </div>
                  <div className="min-w-0">
                    <dt className="text-sm text-muted-foreground">{imports("fields.payment_reference")}</dt>
                    <dd className="mt-1 wrap-anywhere">{member.payment_reference || imports("absentReference")}</dd>
                  </div>
                </dl>
              </section>
            ))}
          </div>

          {result.evidence && (
            <section aria-labelledby="recorded-evidence" className="mt-8 max-w-2xl">
              <h2 className="text-lg font-semibold" id="recorded-evidence">{t("evidence.title")}</h2>
              <p className="mt-2 text-muted-foreground">{t("evidence.description")}</p>
              <dl className="mt-4 flex flex-wrap gap-x-8 gap-y-3 border-y border-border py-4">
                <div className="min-w-0">
                  <dt className="text-sm text-muted-foreground">{imports("fields.payment_reference")}</dt>
                  <dd className="mt-1 font-semibold wrap-anywhere">{result.evidence.payment_reference}</dd>
                  <dd className="mt-1 text-sm text-muted-foreground">
                    {t("evidence.referenceCounts", {
                      internal: result.evidence.internal_reference_count,
                      external: result.evidence.external_reference_count,
                    })}
                  </dd>
                </div>
                <div className="min-w-0">
                  <dt className="text-sm text-muted-foreground">{t("amount")}</dt>
                  <dd className="mt-1 font-semibold tabular-nums wrap-anywhere">
                    {formatMoney(result.evidence.amount, result.evidence.currency, locale)}
                  </dd>
                </div>
              </dl>
            </section>
          )}
        </>
      ) : (
        <p className="mt-5 text-destructive" role="alert">{t(result === null ? "detailNotFound" : "unavailable")}</p>
      )}
    </main>
  );
}
