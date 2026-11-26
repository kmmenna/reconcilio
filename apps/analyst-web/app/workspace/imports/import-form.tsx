"use client";

import { correlatedFetch, workflowURL } from "@/lib/correlation";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { FormSelect } from "@/components/ui/form-select";
import { csvColumns, type FinancialAccount, type ImportView, type ImportProblem, type Source } from "@/lib/imports";
import type importMessages from "@/messages/imports.en.json";
import { CoverageOverlap } from "./coverage-overlap";
import { CSVFileField } from "./csv-file-field";
import { ImportIssues } from "./import-issues";

type Field = keyof typeof importMessages.fields;
type ProblemCode = keyof typeof importMessages.problems;

const fromCSV = "from-csv";

export function ImportForm({ sources, financialAccounts }: { sources: Source[]; financialAccounts: FinancialAccount[] }) {
  const t = useTranslations("Imports");
  const router = useRouter();
  const [selection, setSelection] = useState("");
  const mixed = selection === fromCSV;
  const [pending, setPending] = useState(false);
  const [problem, setProblem] = useState<ImportProblem | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const body = new FormData(event.currentTarget);
    if (mixed) body.delete("financial_account_id");
    if (!selection) {
      setProblem({
        code: "invalid_import_selection",
        issues: [{ field: "financial_account_id", reason: "invalid_financial_account" }],
      });
      return;
    }
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
      const workflow = crypto.randomUUID();
      const response = await correlatedFetch("/api/imports", { method: "POST", body }, workflow);
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
      const preview = await response.json() as ImportView;
      router.push(workflowURL(`/workspace/imports/${encodeURIComponent(preview.id)}`, workflow));
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
            <FormSelect
              {...fieldError("source_id")}
              disabled={pending}
              id="source_id"
              name="source_id"
              placeholder={t("chooseSource")}
              options={sources.map(({ id, name }) => ({ value: id, label: name }))}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="financial_account_id">{t("fields.financial_account_id")}</Label>
            <FormSelect
              {...fieldError("financial_account_id")}
              disabled={pending}
              id="financial_account_id"
              name="financial_account_id"
              placeholder={t("chooseAccount")}
              options={[
                { value: fromCSV, label: t("useCSVAccountCodes") },
                ...financialAccounts.map(({ id, name }) => ({ value: id, label: name })),
              ]}
              value={selection}
              onValueChange={setSelection}
              required
            />
          </div>
        </div>

        <div className="space-y-3 border-t border-border pt-5">
          <CSVFileField
            describedBy={[fieldError("file")["aria-describedby"], mixed && "csv-account-help"].filter(Boolean).join(" ")}
            disabled={pending}
            invalid={fieldError("file")["aria-invalid"]}
          />
          {mixed && <p className="text-sm text-muted-foreground" id="csv-account-help">{t("mixedHelp")}</p>}
          <details className="text-sm text-muted-foreground">
            <summary className="w-fit cursor-pointer text-primary underline-offset-4 hover:underline">{t("fileFormat")}</summary>
            <p className="mt-2">{t("fileFormatHelp")}</p>
            <code className="mt-2 block rounded-md bg-accent p-3 text-xs text-foreground wrap-anywhere">
              {(mixed ? [...csvColumns, "financial_account"] : csvColumns).join(",")}
            </code>
          </details>
        </div>
      </fieldset>

      {problem && (
        <div className="mt-5 rounded-md border border-destructive/35 p-3 text-sm text-destructive" id="import-errors" role="alert">
          <p className="font-semibold">
            {t(t.has(`problems.${problem.code as ProblemCode}`) ? `problems.${problem.code as ProblemCode}` : "problems.preview_unavailable")}
          </p>
          {problem.context?.coverage && <CoverageOverlap coverage={problem.context.coverage} />}
          <ImportIssues issues={problem.issues} />
        </div>
      )}
      <Button className="mt-6 h-11" disabled={pending} type="submit">
        {pending ? t("uploading") : t("showPreview")}
      </Button>
    </form>
  );
}
