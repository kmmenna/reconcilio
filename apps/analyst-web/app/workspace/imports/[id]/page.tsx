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
  const codes = new Map(accounts.map(({ id, code }) => [id, code]));
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
            <dl className="grid grid-cols-2 gap-x-5 gap-y-4 desktop:grid-cols-5">
              {([
                { field: "source_id", value: preview.source.name },
                { field: "source_side", value: <SourceSideLabel side={preview.source.side}>{t(`sides.${preview.source.side}`)}</SourceSideLabel> },
                { field: "financial_account_id", value: accounts.map((account) => (
                  <div key={account.id}>
                    <span>{account.name}</span>
                    <span className="block font-mono text-xs font-normal text-muted-foreground">{account.code}</span>
                  </div>
                )) },
                { field: "period_start", value: <time dateTime={preview.period_start}>{date(preview.period_start)}</time> },
                { field: "period_end", value: <time dateTime={preview.period_end}>{date(preview.period_end)}</time> },
              ] as const).map(({ field, value }) => (
                <div key={field} className={`min-w-0 ${field === "financial_account_id" ? "col-span-2 desktop:col-span-1" : ""}`}>
                  <dt className="text-sm text-muted-foreground">{mixed && field === "financial_account_id" ? t("multipleAccounts") : t(`fields.${field}`)}</dt>
                  <dd className="mt-1 space-y-2 font-semibold wrap-anywhere">{value}</dd>
                </div>
              ))}
              <div className="col-span-full min-w-0 border-t border-border pt-3">
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
              <table aria-labelledby="preview-rows" className="block w-full rounded-xl border border-border bg-card text-left text-sm desktop:table" role="table">
                <thead className="sr-only bg-accent desktop:not-sr-only" role="rowgroup">
                  <tr role="row">
                    {columns.map((column) => (
                      <th className={`px-4 py-3 font-semibold ${column === "amount" ? "text-right" : ""}`} key={column} role="columnheader" scope="col">
                        {t(`fields.${column}`)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="block desktop:table-row-group" role="rowgroup">
                  {preview.rows.map((row) => (
                    <tr
                      className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-3 gap-y-1 border-t border-border px-4 py-3 first:border-t-0 desktop:table-row desktop:align-top desktop:first:border-t"
                      key={row.line}
                      role="row"
                    >
                      <td className="col-span-2 row-start-2 text-muted-foreground tabular-nums desktop:px-4 desktop:py-3 desktop:whitespace-nowrap desktop:text-foreground" role="cell">
                        <time dateTime={row.transaction_date}>{date(row.transaction_date)}</time>
                      </td>
                      <td className="col-start-1 row-start-1 font-medium whitespace-pre-wrap wrap-anywhere desktop:max-w-sm desktop:px-4 desktop:py-3 desktop:font-normal" role="cell">
                        {row.description}
                      </td>
                      <td className="col-start-2 row-start-1 text-right font-medium tabular-nums wrap-anywhere desktop:px-4 desktop:py-3 desktop:whitespace-nowrap" role="cell">
                        {formatMoney(row.amount, row.currency, locale)}
                      </td>
                      {mixed && (
                        <td className="col-span-2 mt-1 text-muted-foreground wrap-anywhere desktop:mt-0 desktop:max-w-40 desktop:px-4 desktop:py-3" role="cell">
                          <span className="mr-2 desktop:hidden">{t("fields.financial_account")}</span>
                          <span className="font-mono text-xs">{codes.get(row.financial_account_id ?? "")}</span>
                        </td>
                      )}
                      <td className="col-span-2 mt-1 text-muted-foreground wrap-anywhere desktop:mt-0 desktop:max-w-40 desktop:px-4 desktop:py-3" role="cell">
                        <span className="mr-2 desktop:hidden">{t("fields.source_reference")}</span>
                        {row.source_reference || t("absentReference")}
                      </td>
                      <td className="col-span-2 text-muted-foreground wrap-anywhere desktop:max-w-40 desktop:px-4 desktop:py-3" role="cell">
                        <span className="mr-2 desktop:hidden">{t("fields.payment_reference")}</span>
                        {row.payment_reference || t("absentReference")}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
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
