"use client";

import { correlatedFetch } from "@/lib/correlation";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useId, useState, useTransition, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FormSelect } from "@/components/ui/form-select";
import type { FinancialAccount, SourceEntry } from "@/lib/imports";
import type messages from "@/messages/catalog.en.json";

type Entry = FinancialAccount | SourceEntry;
type Collection = "financial-accounts" | "sources";
type Problem = keyof typeof messages.problems;

export function CatalogSection({ collection, entries }: { collection: Collection; entries: Entry[] | null | undefined }) {
  const t = useTranslations("Catalog");
  const sides = useTranslations("Imports.sides");
  const heading = useId();
  const router = useRouter();
  const [editor, setEditor] = useState<string | null>(null);
  const [status, setStatus] = useState("");
  const [refreshing, startRefresh] = useTransition();
  const accounts = collection === "financial-accounts";

  function openEditor(id: string | null) {
    setStatus("");
    setEditor(id);
  }

  function saved() {
    setEditor(null);
    setStatus(t("saved"));
    startRefresh(() => router.refresh());
  }

  return (
    <section id={collection} aria-labelledby={heading} aria-busy={refreshing} className="min-w-0">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold" id={heading}>{t(accounts ? "accounts" : "sources")}</h2>
        {entries && editor !== "new" && (
          <Button className="min-h-11 whitespace-normal" disabled={refreshing} onClick={() => openEditor("new")} type="button">
            {t(accounts ? "addAccount" : "addSource")}
          </Button>
        )}
      </div>
      {status && <p className="mt-2 text-sm text-primary" role="status">{status}</p>}
      {editor === "new" && (
        <CatalogEditor collection={collection} onCancel={() => openEditor(null)} onSaved={saved} />
      )}
      {entries?.length ? (
        <ul className="mt-3 divide-y divide-border rounded-xl border border-border bg-card">
          {entries.map((entry) => (
            <li className="px-4 py-3" key={entry.id}>
              <div className="flex items-center justify-between gap-4">
                <div className="min-w-0">
                  <p className="font-semibold wrap-anywhere">{entry.name}</p>
                  {"code" in entry ? (
                    <p className="mt-0.5 font-mono text-xs text-muted-foreground wrap-anywhere">{entry.code}</p>
                  ) : (
                    <p className="mt-0.5 text-sm text-muted-foreground">{sides(entry.side)}</p>
                  )}
                </div>
                {editor !== entry.id && (
                  <Button aria-label={t("editNamed", { name: entry.name })} className="h-11" disabled={refreshing} onClick={() => openEditor(entry.id)} type="button" variant="ghost">
                    {t("edit")}
                  </Button>
                )}
              </div>
              {editor === entry.id && (
                <CatalogEditor collection={collection} entry={entry} onCancel={() => openEditor(null)} onSaved={saved} />
              )}
            </li>
          ))}
        </ul>
      ) : entries ? (
        <p className="mt-4 text-sm text-muted-foreground">{t(accounts ? "emptyAccounts" : "emptySources")}</p>
      ) : (
        <p className="mt-4 text-sm text-destructive" role="alert">{t("problems.catalog_unavailable")}</p>
      )}
    </section>
  );
}

function CatalogEditor({ collection, entry, onCancel, onSaved }: {
  collection: Collection;
  entry?: Entry;
  onCancel: () => void;
  onSaved: () => void;
}) {
  const t = useTranslations("Catalog");
  const sides = useTranslations("Imports.sides");
  const router = useRouter();
  const field = useId();
  const [pending, setPending] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);
  const accounts = collection === "financial-accounts";
  const source = entry && "side" in entry ? entry : undefined;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const values = new FormData(event.currentTarget);
    setPending(true);
    setProblem(null);
    try {
      const suffix = entry ? `/${encodeURIComponent(entry.id)}` : "";
      const response = await correlatedFetch(`/api/catalog/${collection}${suffix}`, {
        method: entry ? "PATCH" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(Object.fromEntries(values)),
      });
      if (response.status === 401) {
        router.replace("/login");
        return;
      }
      if (!response.ok) {
        const { code } = await response.json() as { code?: Problem };
        setProblem(code && t.has(`problems.${code}`) ? code : "catalog_unavailable");
        if (code === "source_side_locked") router.refresh();
        return;
      }
      onSaved();
    } catch {
      setProblem("catalog_unavailable");
    } finally {
      setPending(false);
    }
  }

  return (
    <form aria-busy={pending} className={entry ? "mt-3 border-t border-border pt-4" : "mt-3 rounded-xl border border-border bg-card p-4"} onSubmit={submit}>
      <fieldset className="space-y-3" disabled={pending}>
        <div className="space-y-2">
          <Label htmlFor={`${field}-name`}>{t("name")}</Label>
          <Input autoFocus className="h-11 shadow-none" defaultValue={entry?.name} id={`${field}-name`} name="name" required />
        </div>
        {accounts && !entry && (
          <div className="space-y-2">
            <Label htmlFor={`${field}-code`}>{t("code")}</Label>
            <Input aria-describedby={`${field}-code-help`} className="h-11 shadow-none" id={`${field}-code`} name="code" required />
            <p className="text-sm text-muted-foreground" id={`${field}-code-help`}>{t("codeHelp")}</p>
          </div>
        )}
        {!accounts && (
          <div className="space-y-2">
            <Label htmlFor={`${field}-side`}>{t("side")}</Label>
            <FormSelect
              aria-describedby={source?.side_editable === false ? `${field}-side-help` : undefined}
              defaultValue={source?.side ?? "internal"}
              disabled={pending || source?.side_editable === false}
              id={`${field}-side`}
              key={source?.side_editable === false ? source.side : "editable"}
              name="side"
              options={[
                { value: "internal", label: sides("internal") },
                { value: "external", label: sides("external") },
              ]}
              required
            />
            {source?.side_editable === false && <p className="text-sm text-muted-foreground" id={`${field}-side-help`}>{t("sideLocked")}</p>}
          </div>
        )}
        {problem && <p className="text-sm text-destructive" role="alert">{t(`problems.${problem}`)}</p>}
        <div className="flex flex-wrap gap-2">
          <Button className="h-11" type="submit">{t(pending ? "saving" : "save")}</Button>
          <Button className="h-11" onClick={onCancel} type="button" variant="outline">{t("cancel")}</Button>
        </div>
      </fieldset>
    </form>
  );
}
