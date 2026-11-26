"use client";

import { useTranslations } from "next-intl";

import type { ImportIssue } from "@/lib/imports";
import type importMessages from "@/messages/imports.en.json";

type Field = keyof typeof importMessages.fields;
type Reason = keyof typeof importMessages.reasons;

export function ImportIssues({ issues }: { issues?: ImportIssue[] }) {
  const t = useTranslations("Imports");
  if (!issues?.length) return null;

  return (
    <ul className="mt-2 space-y-2">
      {issues.map(({ line, field, reason, value }, index) => {
        const fieldKey = `fields.${field as Field}` as const;
        const reasonKey = !line && reason === "invalid_date" ? "reasons.invalid_form_date" : `reasons.${reason as Reason}` as const;
        const label = t.has(fieldKey) ? t(fieldKey) : field;
        const message = value && field === "financial_account" && reason === "unknown_financial_account"
          ? t("reasons.unknown_financial_account_code", { code: value })
          : t(t.has(reasonKey) ? reasonKey : "reasons.unknown");
        return (
          <li className="wrap-anywhere" key={`${line}-${field}-${reason}-${index}`}>
            <span className="font-medium">{line ? t("errorLine", { line, field: label }) : label}</span>
            <span className="block">{message}</span>
          </li>
        );
      })}
    </ul>
  );
}
