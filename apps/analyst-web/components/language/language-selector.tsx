"use client";

import { CheckIcon, ChevronDownIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { DropdownMenu } from "radix-ui";
import { useEffect, useId, useOptimistic, useRef, useState, useTransition } from "react";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { isLocale, type LanguagePreference } from "@/i18n/locales";
import { correlatedFetch } from "@/lib/correlation";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

export type LanguagePresentation = "select" | "menu";

export function LanguageSelector({ preference, presentation }: {
  preference: LanguagePreference;
  presentation: LanguagePresentation;
}) {
  const router = useRouter();
  const t = useTranslations("Language");
  const id = useId();
  const [pending, startTransition] = useTransition();
  const [selected, setSelected] = useOptimistic(preference);
  const [error, setError] = useState<string | null>(null);
  const menuTrigger = useRef<HTMLButtonElement>(null);
  const errorMessage = useRef<HTMLParagraphElement>(null);
  const message = useProblemMessage(error, "language_unavailable");
  const errorId = message ? `${id}-error` : undefined;
  const options = [
    { value: "auto", label: t("automatic") },
    { value: "pt-BR", label: "Português (Brasil)", lang: "pt-BR" },
    { value: "en", label: "English", lang: "en" },
  ];

  useEffect(() => {
    if (message && document.activeElement === menuTrigger.current) {
      errorMessage.current?.scrollIntoView({ block: "nearest" });
    }
  }, [message]);

  function changeLanguage(locale: string) {
    if (locale === selected || (locale !== "auto" && !isLocale(locale))) return;
    setError(null);
    startTransition(async () => {
      setSelected(locale);
      try {
        const response = await correlatedFetch("/api/preferences/language", {
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
      {presentation === "menu" ? (
        <DropdownMenu.Root modal={false}>
          <DropdownMenu.Trigger asChild>
            <Button
              variant="link"
              size="sm"
              className="has-[>svg]:px-0"
              aria-describedby={errorId}
              ref={menuTrigger}
            >
              {t("label")}
              <ChevronDownIcon aria-hidden="true" className="size-3.5" />
            </Button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content
              align="end"
              side="top"
              sideOffset={6}
              collisionPadding={16}
              className="z-50 w-44 max-h-(--radix-dropdown-menu-content-available-height) max-w-(--radix-dropdown-menu-content-available-width) overflow-y-auto rounded-md border border-border bg-card p-1 text-foreground outline-none"
            >
              <DropdownMenu.RadioGroup value={selected} onValueChange={changeLanguage}>
                {options.map(({ value, label, lang }) => (
                  <DropdownMenu.RadioItem
                    className="flex min-h-10 cursor-pointer items-center justify-between gap-2 rounded-md px-3 py-2 text-sm data-highlighted:bg-accent focus-visible:-outline-offset-2 data-disabled:cursor-wait data-disabled:opacity-60"
                    disabled={pending}
                    key={value}
                    value={value}
                  >
                    <span lang={lang}>{label}</span>
                    <DropdownMenu.ItemIndicator>
                      <CheckIcon aria-hidden="true" className="size-4 text-primary" />
                    </DropdownMenu.ItemIndicator>
                  </DropdownMenu.RadioItem>
                ))}
              </DropdownMenu.RadioGroup>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      ) : (
        <div className="flex items-center gap-2.5">
          <Label className="text-muted-foreground" htmlFor={id}>{t("label")}</Label>
          <NativeSelect
            aria-describedby={errorId}
            className="h-11 bg-card pr-9 shadow-none"
            disabled={pending}
            id={id}
            value={selected}
            onChange={(event) => changeLanguage(event.currentTarget.value)}
          >
            {options.map(({ value, label, lang }) => (
              <NativeSelectOption key={value} value={value} lang={lang}>{label}</NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      )}
      {message && (
        <p className="mt-2 max-w-56 scroll-mb-3 text-sm text-destructive" id={errorId} ref={errorMessage} role="alert">
          {message}
        </p>
      )}
    </div>
  );
}
