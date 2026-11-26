import type { Locale } from "@/i18n/locales";

const moneyFormats = new Map<string, Intl.NumberFormat>();
const dateFormats = new Map<Locale, Intl.DateTimeFormat>();
const timestampFormats = new Map<Locale, Intl.DateTimeFormat>();

export function formatMoney(amount: string, currency: string, locale: Locale) {
  const decimalPoint = amount.indexOf(".");
  const fractionDigits = decimalPoint === -1 ? 0 : amount.length - decimalPoint - 1;
  const key = `${locale}:${currency}:${fractionDigits}`;
  let format = moneyFormats.get(key);
  if (!format) {
    format = new Intl.NumberFormat(locale, {
      style: "currency",
      currency,
      minimumFractionDigits: fractionDigits,
      maximumFractionDigits: fractionDigits,
    });
    moneyFormats.set(key, format);
  }
  // Intl accepts exact decimal strings without converting through a floating-point number.
  return format.format(amount as Intl.StringNumericLiteral);
}

export function formatTransactionDate(date: string, locale: Locale) {
  let format = dateFormats.get(locale);
  if (!format) {
    format = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" });
    dateFormats.set(locale, format);
  }
  return format.format(new Date(`${date}T00:00:00Z`));
}

// UTC keeps recorded completion times consistent across browsers.
export function formatTimestamp(timestamp: string, locale: Locale) {
  let format = timestampFormats.get(locale);
  if (!format) {
    format = new Intl.DateTimeFormat(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
      timeZone: "UTC",
      timeZoneName: "short",
    });
    timestampFormats.set(locale, format);
  }
  return format.format(new Date(timestamp));
}
