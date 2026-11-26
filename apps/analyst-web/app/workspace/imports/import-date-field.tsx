"use client";

import { DayPicker } from "@daypicker/react";
import { ptBR } from "@daypicker/react/locale/pt-BR";
import { CalendarDaysIcon } from "lucide-react";
import { useFormatter, useLocale, useTranslations } from "next-intl";
import { Popover } from "radix-ui";
import { useLayoutEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Locale } from "@/i18n/locales";

const isoDatePattern = /^\d{4}-\d{2}-\d{2}$/;

function maskDate(value: string, locale: Locale) {
  if (isoDatePattern.test(value.trim())) return fromISODate(value.trim(), locale);

  const [first, second, ...year] = value.replace(/[^\d/]/g, "").split("/");
  if (first.length > 2) {
    const digits = [first, second, ...year].join("").slice(0, 8);
    return [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4)].filter(Boolean).join("/");
  }
  if (second === undefined) return first;
  const rest = [second.slice(2), ...year].join("").slice(0, 4);
  const prefix = `${first}/${second.slice(0, 2)}`;
  return rest || year.length ? `${prefix}/${rest}` : prefix;
}

function maskedCaret(raw: string, masked: string, offset: number) {
  if (offset === raw.length || isoDatePattern.test(raw.trim())) return masked.length;
  const prefix = raw.slice(0, offset);
  const digits = prefix.replace(/\D/g, "").length;
  let caret = 0;
  let seen = 0;
  while (caret < masked.length && seen < digits) {
    if (/\d/.test(masked[caret])) seen += 1;
    caret += 1;
  }
  return prefix.endsWith("/") && masked[caret] === "/" ? caret + 1 : caret;
}

function toISODate(value: string, locale: Locale) {
  const text = value.trim();
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

function calendarDate(iso: string) {
  if (!iso) return undefined;
  const date = new Date(`${iso}T00:00:00Z`);
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === iso ? date : undefined;
}

function browserToday() {
  const local = new Date();
  // The picker uses UTC as a calendar anchor. Today still comes from the browser's civil date.
  return new Date(Date.UTC(local.getFullYear(), local.getMonth(), local.getDate()));
}

export function ImportDateField({ field, invalid }: {
  field: "period_start" | "period_end";
  invalid?: boolean;
}) {
  const t = useTranslations("Imports");
  const locale = useLocale();
  const format = useFormatter();
  const [draft, setDraft] = useState({ value: "", locale });
  const [calendarToday, setCalendarToday] = useState<Date | null>(null);
  const input = useRef<HTMLInputElement>(null);
  const caret = useRef<number | null>(null);
  const label = t(`fields.${field}`);

  useLayoutEffect(() => {
    if (caret.current === null) return;
    input.current?.setSelectionRange(caret.current, caret.current);
    caret.current = null;
  }, [draft]);

  // Keep the parsed date when the analyst switches languages while editing.
  const iso = toISODate(draft.value, draft.locale);
  const selectedDay = calendarDate(iso);
  const value = iso && draft.locale !== locale ? fromISODate(iso, locale) : draft.value;

  return (
    <div className="space-y-2">
      <Label htmlFor={field}>{label}</Label>
      <Popover.Root open={calendarToday !== null} onOpenChange={(open) => setCalendarToday(open ? browserToday() : null)}>
        <Popover.Anchor asChild>
          <div className="relative max-w-55">
            <Input
              aria-describedby={invalid ? "import-errors" : undefined}
              aria-invalid={invalid}
              autoComplete="off"
              className="h-11 pr-12 shadow-none"
              id={field}
              inputMode="numeric"
              placeholder={t("dateFormat")}
              ref={input}
              type="text"
              value={value}
              required
              onChange={(event) => {
                const { value: raw, selectionStart } = event.currentTarget;
                const masked = maskDate(raw, locale);
                caret.current = maskedCaret(raw, masked, selectionStart ?? raw.length);
                setDraft({ value: masked, locale });
              }}
              onBlur={() => {
                if (iso) setDraft({ value: fromISODate(iso, locale), locale });
              }}
            />
            <Popover.Trigger asChild>
              <Button
                aria-label={t("calendar.open", { field: label })}
                className="absolute top-1 right-1 size-9 p-0 text-primary focus-visible:-outline-offset-2"
                size="icon"
                type="button"
                variant="ghost"
              >
                <CalendarDaysIcon aria-hidden="true" />
              </Button>
            </Popover.Trigger>
          </div>
        </Popover.Anchor>
        <Popover.Portal>
          <Popover.Content
            align="start"
            aria-label={t("calendar.title", { field: label })}
            className="z-50 max-h-(--radix-popover-content-available-height) overflow-auto rounded-lg border border-border bg-card p-3 text-foreground shadow-md"
            collisionPadding={8}
            onOpenAutoFocus={(event) => event.preventDefault()}
            sideOffset={6}
          >
            {calendarToday && (
              <DayPicker
                autoFocus
                className="import-calendar text-sm"
                defaultMonth={selectedDay ?? calendarToday}
                formatters={{
                  formatCaption: (month) => format.dateTime(month, { month: "long", year: "numeric", timeZone: "UTC" }),
                }}
                labels={{
                  labelNav: () => t("calendar.navigation"),
                  labelPrevious: () => t("calendar.previous"),
                  labelNext: () => t("calendar.next"),
                  labelDayButton: (day, modifiers) => {
                    const date = format.dateTime(day, { dateStyle: "full", timeZone: "UTC" });
                    return modifiers.today ? t("calendar.today", { date }) : date;
                  },
                }}
                locale={locale === "pt-BR" ? ptBR : undefined}
                mode="single"
                navLayout="around"
                onSelect={(day) => {
                  if (!day) return;
                  const selected = day.toISOString().slice(0, 10);
                  setDraft({ value: fromISODate(selected, locale), locale });
                  setCalendarToday(null);
                }}
                required
                selected={selectedDay}
                timeZone="UTC"
                today={calendarToday}
              />
            )}
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
      <input type="hidden" name={field} value={iso} />
    </div>
  );
}
