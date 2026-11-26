"use client";

import { CheckIcon, ChevronDownIcon, GlobeIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { DropdownMenu } from "radix-ui";
import { useEffect, useId, useOptimistic, useRef, useState, useTransition } from "react";

import { Button } from "@/components/ui/button";
import { isLocale, type LanguagePreference } from "@/i18n/locales";
import { correlatedFetch } from "@/lib/correlation";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

export function LanguageSelector({ preference, side }: {
  preference: LanguagePreference;
  side: "top" | "bottom";
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
      <DropdownMenu.Root modal={false}>
        <DropdownMenu.Trigger asChild>
          <Button
            variant="link"
            size="sm"
            className="has-[>svg]:px-0"
            aria-describedby={errorId}
            ref={menuTrigger}
          >
            <GlobeIcon aria-hidden="true" className="size-4" />
            {t("label")}
            <ChevronDownIcon aria-hidden="true" className="size-3.5" />
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            side={side}
            sideOffset={6}
            collisionPadding={16}
            className="choice-menu w-44 max-h-(--radix-dropdown-menu-content-available-height) max-w-(--radix-dropdown-menu-content-available-width)"
          >
            <DropdownMenu.RadioGroup value={selected} onValueChange={changeLanguage}>
              {options.map(({ value, label, lang }) => (
                <DropdownMenu.RadioItem
                  className="choice-option"
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
      {message && (
        <p className="mt-2 max-w-56 scroll-mb-3 text-sm text-destructive" id={errorId} ref={errorMessage} role="alert">
          {message}
        </p>
      )}
    </div>
  );
}
