import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { LocalTimestamp } from "@/components/local-timestamp";
import { SourceSideLabel } from "@/components/source-side-label";
import { Button } from "@/components/ui/button";
import { readWorkflowID } from "@/lib/auth";
import { workflowURL } from "@/lib/correlation";
import type { ImportView } from "@/lib/imports";
import { formatMoney, formatTransactionDate } from "@/lib/transaction-format";
import { getWorkspaceToken } from "@/lib/workspace";
import { readWorkspaceResource } from "@/lib/workspace-api";
import { ConfirmImport } from "./confirm-import";
import { ImportConflicts } from "./import-conflicts";

export default async function PreviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const token = await getWorkspaceToken();
  const [preview, t, locale, workflow] = await Promise.all([
    readWorkspaceResource<ImportView>(`/api/imports/${encodeURIComponent(id)}`, token),
    getTranslations("Imports"),
    getLocale(),
    readWorkflowID(),
  ]);
  const confirmed = preview?.status === "confirmed";
  const canceled = preview?.status === "canceled";
  const isPending = preview?.status === "pending";
  const settledAt = confirmed ? preview.confirmed_at : canceled ? preview.canceled_at : null;
  const date = (value: string) => formatTransactionDate(value, locale);
  const accounts = preview?.financial_accounts ?? (preview?.financial_account ? [preview.financial_account] : []);
  const mixed = accounts.length > 1;
  const accountsByID = new Map(accounts.map((account) => [account.id, account]));
  const coverage = preview?.account_coverage?.map((period) => {
    const account = accountsByID.get(period.financial_account_id);
    return { ...period, key: period.financial_account_id, accounts: account ? [account] : [] };
  }) ?? (preview ? [{ key: preview.id, accounts, period_start: preview.period_start, period_end: preview.period_end }] : []);
  const columns = [
    "transaction_date", "description", "amount", ...(mixed ? ["financial_account"] as const : []), "source_reference", "payment_reference",
  ] as const;

  return (
    <main className="workspace">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="page-title">{t(confirmed ? "confirmedTitle" : canceled ? "canceledTitle" : "previewTitle")}</h1>
          {preview?.status === "pending" && <p className="mt-3 max-w-prose text-muted-foreground">{t("previewIntro")}</p>}
          {confirmed && <p className="mt-3 text-muted-foreground">{t("importedCount", { count: preview.transaction_count })}</p>}
          {canceled && <p className="mt-3 text-muted-foreground">{t("canceledSummary")}</p>}
        </div>
        {preview && (
          <Button asChild variant="outline" className="h-11">
            <Link href="/workspace/imports">{t("anotherImport")}</Link>
          </Button>
        )}
      </div>
      {preview ? (
        <>
          <div className="mt-7 rounded-xl border border-border bg-card p-5 desktop:p-6">
            {isPending && (
              <p className="mb-4 w-fit rounded-full bg-accent px-3 py-1 text-sm font-medium text-primary">
                {t("pending")}
              </p>
            )}
            <div className="grid gap-4 desktop:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
              <dl className="grid grid-cols-2 gap-x-5">
                <div className="min-w-0">
                  <dt className="text-sm text-muted-foreground">{t("fields.source_id")}</dt>
                  <dd className="mt-1 font-semibold wrap-anywhere">{preview.source.name}</dd>
                </div>
                <div className="min-w-0">
                  <dt className="text-sm text-muted-foreground">{t("fields.source_side")}</dt>
                  <dd className="mt-1 font-semibold">
                    <SourceSideLabel side={preview.source.side}>{t(`sides.${preview.source.side}`)}</SourceSideLabel>
                  </dd>
                </div>
              </dl>
              <section aria-labelledby="import-coverage">
                <h2 className="text-sm text-muted-foreground" id="import-coverage">{t("coverageTitle")}</h2>
                <ul className="mt-1 divide-y divide-border">
                  {coverage.map(({ key, accounts, period_start, period_end }) => (
                    <li className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 py-2 first:pt-0 last:pb-0" key={key}>
                      <div className="space-y-1 wrap-anywhere">
                        {accounts.map((account) => (
                          <div key={account.id}>
                            <span className="font-semibold">{account.name}</span>
                            <span className="block font-mono text-xs text-muted-foreground">{account.code}</span>
                          </div>
                        ))}
                      </div>
                      <p className="text-sm font-medium tabular-nums">
                        <time dateTime={period_start}>{date(period_start)}</time>
                        {period_end !== period_start && <>{" – "}<time dateTime={period_end}>{date(period_end)}</time></>}
                      </p>
                    </li>
                  ))}
                </ul>
              </section>
            </div>
            <dl className="mt-4 space-y-4 border-t border-border pt-3">
              <div className="min-w-0">
                <dt className="text-sm text-muted-foreground desktop:inline">{t(isPending ? "previewId" : "importId")}</dt>{" "}
                <dd className="font-mono text-xs wrap-anywhere desktop:inline">{preview.id}</dd>
              </div>
              {settledAt && (
                <div className="col-span-full">
                  <dt className="text-sm text-muted-foreground">{t(confirmed ? "confirmedAt" : "canceledAt")}</dt>
                  <dd className="mt-1 font-semibold tabular-nums">
                    <LocalTimestamp value={settledAt} />
                  </dd>
                </div>
              )}
            </dl>
          </div>

          {isPending && <ConfirmImport key={preview.id} id={preview.id} accounts={accounts} workflow={workflow} initialConflicts={preview.conflicts ?? []} />}
          {!isPending && preview.conflicts?.length ? (
            <section className="mt-8" aria-labelledby="recorded-decision">
              <h2 className="text-lg font-semibold" id="recorded-decision">{t("recordedDecision")}</h2>
              <p className="mt-1 mb-4 max-w-prose text-muted-foreground">{t(confirmed ? "keptSummary" : "canceledConflictSummary")}</p>
              <ImportConflicts accounts={accounts} conflicts={preview.conflicts} />
            </section>
          ) : null}

          {confirmed && (
            <section className="mt-8" aria-labelledby="processing">
              <h2 className="text-lg font-semibold" id="processing">{t("processing")}</h2>
              <ul className="mt-3 divide-y divide-border rounded-xl border border-border bg-card">
                {preview.jobs.map((job) => (
                  <li className="flex flex-wrap items-center justify-between gap-3 px-4 py-3" key={job.job_id}>
                    <span className="font-semibold wrap-anywhere">{job.financial_account.name}</span>
                    <Button asChild variant="outline" className="h-11">
                      <Link href={workflowURL(`/workspace/jobs/${job.job_id}`, workflow)}>{t("viewJob")}</Link>
                    </Button>
                  </li>
                ))}
              </ul>
            </section>
          )}
          {isPending && (
            <section className="mt-8" aria-labelledby="preview-rows">
              <div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4">
                <h2 className="text-lg font-semibold" id="preview-rows">{t("transactions")}</h2>
                <p className="text-sm text-muted-foreground">{t("rowCount", { count: preview.rows.length })}</p>
              </div>
              <div className="overflow-hidden rounded-xl border border-border bg-card">
                <table aria-labelledby="preview-rows" className="block w-full text-left text-sm lg:table" role="table">
                  <thead className="sr-only bg-accent lg:not-sr-only" role="rowgroup">
                    <tr role="row">
                      {columns.map((column) => (
                        <th className={`px-5 py-3 ${column === "amount" ? "text-right" : ""}`} key={column} role="columnheader" scope="col">
                          {t(`fields.${column}`)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="block lg:table-row-group" role="rowgroup">
                    {preview.rows.map((row) => (
                      <tr
                        className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2 border-t border-border p-4 first:border-t-0 lg:table-row lg:align-top lg:first:border-t"
                        key={row.line}
                        role="row"
                      >
                        <td className="col-span-2 row-start-2 text-muted-foreground tabular-nums lg:px-5 lg:py-4 lg:whitespace-nowrap lg:text-foreground" role="cell">
                          <time dateTime={row.transaction_date}>{date(row.transaction_date)}</time>
                        </td>
                        <td className="col-start-1 row-start-1 font-medium whitespace-pre-wrap wrap-anywhere lg:max-w-sm lg:px-5 lg:py-4" role="cell">
                          {row.description}
                        </td>
                        <td className="col-start-2 row-start-1 text-right font-semibold tabular-nums wrap-anywhere lg:px-5 lg:py-4 lg:whitespace-nowrap" role="cell">
                          {formatMoney(row.amount, row.currency, locale)}
                        </td>
                        {mixed && (
                          <td className="col-span-2 mt-1 text-muted-foreground wrap-anywhere lg:mt-0 lg:max-w-40 lg:px-5 lg:py-4" role="cell">
                            <span className="mr-2 lg:hidden">{t("fields.financial_account")}</span>
                            <span className="font-mono text-xs">{accountsByID.get(row.financial_account_id ?? "")?.code}</span>
                          </td>
                        )}
                        <td className="col-span-2 mt-1 text-muted-foreground wrap-anywhere lg:mt-0 lg:max-w-40 lg:px-5 lg:py-4" role="cell">
                          <span className="mr-2 lg:hidden">{t("fields.source_reference")}</span>
                          {row.source_reference || t("absentReference")}
                        </td>
                        <td className="col-span-2 text-muted-foreground wrap-anywhere lg:max-w-40 lg:px-5 lg:py-4" role="cell">
                          <span className="mr-2 lg:hidden">{t("fields.payment_reference")}</span>
                          {row.payment_reference || t("absentReference")}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        </>
      ) : (
        <p className="mt-5 text-destructive" role="alert">
          {t(preview === null ? "problems.preview_not_found" : "problems.preview_unavailable")}
        </p>
      )}
    </main>
  );
}
