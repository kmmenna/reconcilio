"use client";

import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Locale } from "@/i18n/locales";

function toISODate(value: string, locale: Locale) {
  const text = value.trim();
  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) return text;

  const local = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(text);
  if (!local) return "";
  const [, first, second, year] = local;
  const [month, day] = locale === "pt-BR" ? [second, first] : [first, second];
  return `${year}-${month.padStart(2, "0")}-${day.padStart(2, "0")}`;
}

function fromISODate(iso: string, locale: Locale) {
  const [year, month, day] = iso.split("-");
  return locale === "pt-BR" ? `${day}/${month}/${year}` : `${month}/${day}/${year}`;
}

export function ImportDateField({ field, invalid }: {
  field: "period_start" | "period_end";
  invalid?: boolean;
}) {
  const t = useTranslations("Imports");
  const locale = useLocale();
  const [draft, setDraft] = useState({ value: "", locale });

  // Keep the parsed date when the analyst switches languages while editing.
  const iso = toISODate(draft.value, draft.locale);
  const value = iso && draft.locale !== locale ? fromISODate(iso, locale) : draft.value;

  return (
    <div className="space-y-2">
      <Label htmlFor={field}>{t(`fields.${field}`)}</Label>
      <Input
        aria-describedby={invalid ? "import-errors" : undefined}
        aria-invalid={invalid}
        autoComplete="off"
        className="h-11 max-w-55 shadow-none"
        id={field}
        placeholder={t("dateFormat")}
        type="text"
        value={value}
        required
        onChange={(event) => setDraft({ value: event.currentTarget.value, locale })}
        onBlur={() => {
          if (iso) setDraft({ value: fromISODate(iso, locale), locale });
        }}
      />
      <input type="hidden" name={field} value={iso} />
    </div>
  );
}
