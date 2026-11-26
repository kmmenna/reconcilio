"use client";

import { useLocale } from "next-intl";
import { useSyncExternalStore } from "react";

import { formatTimestamp } from "@/lib/transaction-format";

function subscribe() {
  return () => {};
}

function getBrowserTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

function getServerTimeZone() {
  return null;
}

export function LocalTimestamp({ value }: { value: string }) {
  const locale = useLocale();
  // The server cannot determine the browser's time zone.
  const timeZone = useSyncExternalStore(subscribe, getBrowserTimeZone, getServerTimeZone);

  return <time dateTime={value}>{timeZone && formatTimestamp(value, locale, timeZone)}</time>;
}
