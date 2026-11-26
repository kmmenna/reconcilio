"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { csvColumns, type FinancialAccount, type ImportPreview, type ImportProblem, type Source } from "@/lib/imports";
import type importMessages from "@/messages/imports.en.json";
import { ImportDateField } from "./import-date-field";

type Field = keyof typeof importMessages.fields;
type Reason = keyof typeof importMessages.reasons;
type ProblemCode = keyof typeof importMessages.problems;

export function ImportForm({ sources, financialAccounts }: { sources: Source[]; financialAccounts: FinancialAccount[] }) {
  const t = useTranslations("Imports");
  const router = useRouter();
  const [fileName, setFileName] = useState("");
  const [pending, setPending] = useState(false);
  const [problem, setProblem] = useState<ImportProblem | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const body = new FormData(event.currentTarget);
    const file = body.get("file");
    if (!(file instanceof File) || !file.name) {
      setProblem({
        code: "invalid_import_selection",
        issues: [{ field: "file", reason: "missing_file" }],
      });
      return;
    }
    setPending(true);
    setProblem(null);
    try {
      const response = await fetch("/api/imports", { method: "POST", body });
      if (response.status === 401) {
        router.replace("/login");
        return;
      }
      if (response.status === 413) {
        setProblem({ code: "file_too_large" });
        return;
      }
      if (!response.ok) {
        setProblem(await response.json() as ImportProblem);
        return;
      }
      const preview = await response.json() as ImportPreview;
      router.push(`/workspace/imports/${encodeURIComponent(preview.id)}`);
    } catch {
      setProblem({ code: "preview_unavailable" });
    } finally {
      setPending(false);
    }
  }

  function fieldError(field: Field) {
    const invalid = problem?.issues?.some((item) => !item.line && item.field === field);
    return { "aria-invalid": invalid, "aria-describedby": invalid ? "import-errors" : undefined };
  }

  return (
    <form
      action="/api/imports"
      aria-busy={pending}
      className="mt-7 max-w-[44rem] rounded-xl border border-border bg-card p-5 desktop:p-7"
      encType="multipart/form-data"
      method="post"
      noValidate
      onSubmit={submit}
    >
      <fieldset disabled={pending} className="min-w-0 space-y-6">
        <div className="grid gap-5 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="source_id">{t("fields.source_id")}</Label>
            <NativeSelect {...fieldError("source_id")} className="h-11 max-w-full shadow-none" id="source_id" name="source_id" defaultValue="" required>
              <NativeSelectOption value="" disabled>{t("chooseSource")}</NativeSelectOption>
              {sources.map((source) => <NativeSelectOption key={source.id} value={source.id}>{source.name}</NativeSelectOption>)}
            </NativeSelect>
          </div>
          <div className="space-y-2">
            <Label htmlFor="financial_account_id">{t("fields.financial_account_id")}</Label>
            <NativeSelect {...fieldError("financial_account_id")} className="h-11 max-w-full shadow-none" id="financial_account_id" name="financial_account_id" defaultValue="" required>
              <NativeSelectOption value="" disabled>{t("chooseAccount")}</NativeSelectOption>
              {financialAccounts.map((account) => <NativeSelectOption key={account.id} value={account.id}>{account.name}</NativeSelectOption>)}
            </NativeSelect>
          </div>
          <ImportDateField field="period_start" invalid={fieldError("period_start")["aria-invalid"]} />
          <ImportDateField field="period_end" invalid={fieldError("period_end")["aria-invalid"]} />
        </div>

        <div className="space-y-3 border-t border-border pt-5">
          <Label htmlFor="csv-file">{t("fields.file")}</Label>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <input
              {...fieldError("file")}
              accept=".csv,text/csv"
              aria-label={t("fields.file")}
              className="peer sr-only"
              id="csv-file"
              name="file"
              type="file"
              required
              onChange={(event) => setFileName(event.currentTarget.files?.[0]?.name ?? "")}
            />
            <label
              className="inline-flex min-h-11 cursor-pointer items-center rounded-md border border-input px-3 text-sm font-medium hover:bg-accent peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-ring peer-disabled:cursor-wait peer-disabled:opacity-50"
              htmlFor="csv-file"
            >
              {t("chooseFile")}
            </label>
            <span className="min-w-0 text-sm text-muted-foreground wrap-anywhere" aria-live="polite">
              {fileName || t("noFile")}
            </span>
          </div>
          <details className="text-sm text-muted-foreground">
            <summary className="w-fit cursor-pointer text-primary underline-offset-4 hover:underline">{t("fileFormat")}</summary>
            <p className="mt-2">{t("fileFormatHelp")}</p>
            <code className="mt-2 block rounded-md bg-accent p-3 text-xs text-foreground wrap-anywhere">
              {csvColumns.join(",")}
            </code>
          </details>
        </div>
      </fieldset>

      {problem && (
        <div className="mt-5 rounded-md border border-destructive/35 p-3 text-sm text-destructive" id="import-errors" role="alert">
          <p className="font-semibold">
            {t(t.has(`problems.${problem.code as ProblemCode}`) ? `problems.${problem.code as ProblemCode}` : "problems.preview_unavailable")}
          </p>
          {problem.issues?.length ? (
            <ul className="mt-2 space-y-2">
              {problem.issues.map(({ line, field, reason }, index) => {
                const fieldKey = `fields.${field as Field}` as const;
                const reasonKey = !line && reason === "invalid_date" ? "reasons.invalid_form_date" : `reasons.${reason as Reason}` as const;
                const label = t.has(fieldKey) ? t(fieldKey) : field;
                return (
                  <li key={`${line}-${field}-${reason}-${index}`}>
                    <span className="font-medium">{line ? t("errorLine", { line, field: label }) : label}</span>
                    <span className="block">{t(t.has(reasonKey) ? reasonKey : "reasons.unknown")}</span>
                  </li>
                );
              })}
            </ul>
          ) : null}
        </div>
      )}
      <Button className="mt-6 h-11" disabled={pending} type="submit">
        {pending ? t("uploading") : t("showPreview")}
      </Button>
    </form>
  );
}
