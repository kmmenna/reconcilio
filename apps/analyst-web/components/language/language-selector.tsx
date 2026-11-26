"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useId, useOptimistic, useState, useTransition } from "react";

import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { isLocale, type LanguagePreference } from "@/i18n/locales";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

export function LanguageSelector({ preference }: { preference: LanguagePreference }) {
  const router = useRouter();
  const t = useTranslations("Language");
  const id = useId();
  const [pending, startTransition] = useTransition();
  const [selected, setSelected] = useOptimistic(preference);
  const [error, setError] = useState<string | null>(null);
  const message = useProblemMessage(error, "language_unavailable");

  function changeLanguage(locale: string) {
    if (locale !== "auto" && !isLocale(locale)) return;
    setError(null);
    startTransition(async () => {
      setSelected(locale);
      try {
        const response = await fetch("/api/preferences/language", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ locale }),
        });
        if (response.ok) {
          router.refresh();
        } else {
          setError(await readProblemCode(response) ?? "language_unavailable");
        }
      } catch {
        setError("language_unavailable");
      }
    });
  }

  return (
    <div className="min-w-0">
      <div className="flex items-center gap-2.5">
        <Label className="text-muted-foreground" htmlFor={id}>{t("label")}</Label>
        <NativeSelect
          aria-describedby={message ? `${id}-error` : undefined}
          className="h-11 bg-card pr-9 shadow-none"
          disabled={pending}
          id={id}
          value={selected}
          onChange={(event) => changeLanguage(event.currentTarget.value)}
        >
          <NativeSelectOption value="auto">{t("automatic")}</NativeSelectOption>
          <NativeSelectOption value="pt-BR" lang="pt-BR">Português (Brasil)</NativeSelectOption>
          <NativeSelectOption value="en" lang="en">English</NativeSelectOption>
        </NativeSelect>
      </div>
      {message && <p className="mt-2 max-w-56 text-sm text-destructive" id={`${id}-error`} role="alert">{message}</p>}
    </div>
  );
}
