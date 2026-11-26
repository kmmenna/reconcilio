import Link from "next/link";
import { redirect } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";
import { requestAPI } from "@/lib/auth";
import type { ImportPreview } from "@/lib/imports";
import { formatMoney, formatTransactionDate } from "@/lib/transaction-format";
import { getWorkspaceToken } from "@/lib/workspace";

async function readPreview(id: string, token: string): Promise<ImportPreview | null | undefined> {
  const response = await requestAPI(`/api/imports/${encodeURIComponent(id)}`, {
    headers: { Authorization: `Bearer ${token}` },
  }).catch(() => undefined);
  if (response?.status === 401) redirect("/login");
  if (response?.status === 404) return null;
  if (!response?.ok) return undefined;
  return response.json().catch(() => undefined);
}

export default async function PreviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const token = await getWorkspaceToken();
  const [preview, t, locale] = await Promise.all([
    readPreview(id, token),
    getTranslations("Imports"),
    getLocale(),
  ]);
  const date = (value: string) => formatTransactionDate(value, locale);
  const columns = ["transaction_date", "description", "amount", "source_reference", "payment_reference"] as const;

  return (
    <main className="workspace">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="page-title">{t("previewTitle")}</h1>
          {preview && <p className="mt-3 max-w-prose text-muted-foreground">{t("previewIntro")}</p>}
        </div>
        <Button asChild variant="outline" className="h-11">
          <Link href="/workspace/imports">{t("anotherImport")}</Link>
        </Button>
      </div>
      {preview ? (
        <>
          <div className="mt-7 rounded-xl border border-border bg-card p-5 desktop:p-6">
            <p className="mb-4 w-fit rounded-full bg-accent px-3 py-1 text-sm font-medium text-primary">
              {t("pending")}
            </p>
            <dl className="grid grid-cols-2 gap-x-5 gap-y-4 desktop:grid-cols-5">
              {([
                { field: "source_id", value: preview.source.name },
                { field: "source_side", value: t(`sides.${preview.source.side}`) },
                { field: "financial_account_id", value: preview.financial_account.name },
                { field: "period_start", value: <time dateTime={preview.period_start}>{date(preview.period_start)}</time> },
                { field: "period_end", value: <time dateTime={preview.period_end}>{date(preview.period_end)}</time> },
              ] as const).map(({ field, value }) => (
                <div key={field} className={`min-w-0 ${field === "financial_account_id" ? "col-span-2 desktop:col-span-1" : ""}`}>
                  <dt className="text-sm text-muted-foreground">{t(`fields.${field}`)}</dt>
                  <dd className="mt-1 font-semibold wrap-anywhere">{value}</dd>
                </div>
              ))}
              <div className="col-span-full min-w-0 border-t border-border pt-3">
                <dt className="text-sm text-muted-foreground desktop:inline">{t("previewId")}</dt>{" "}
                <dd className="font-mono text-xs wrap-anywhere desktop:inline">{preview.id}</dd>
              </div>
            </dl>
          </div>

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
        </>
      ) : (
        <p className="mt-5 text-destructive" role="alert">
          {t(preview === null ? "problems.preview_not_found" : "problems.preview_unavailable")}
        </p>
      )}
    </main>
  );
}
