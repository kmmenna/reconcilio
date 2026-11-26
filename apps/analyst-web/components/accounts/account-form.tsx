"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useId, useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

const fields = {
  username: {
    jsonName: "username",
    type: "text",
    autoComplete: "off",
    min: 1,
    max: 100,
    error: "invalid_username",
    conflict: "username_taken",
    hint: "usernameHint",
  },
  displayName: {
    jsonName: "display_name",
    type: "text",
    autoComplete: "off",
    min: 1,
    max: 120,
    error: "invalid_display_name",
  },
  password: {
    jsonName: "password",
    type: "password",
    autoComplete: "new-password",
    min: 8,
    max: 512,
    error: "invalid_new_password",
    hint: "passwordHint",
  },
  currentPassword: {
    jsonName: "current_password",
    type: "password",
    autoComplete: "current-password",
    min: 1,
    max: 512,
    error: "invalid_current_password",
    conflict: "incorrect_current_password",
  },
  newPassword: {
    jsonName: "new_password",
    type: "password",
    autoComplete: "new-password",
    min: 8,
    max: 512,
    error: "invalid_new_password",
    hint: "passwordHint",
  },
} as const;

const forms = {
  create: {
    path: "/api/users",
    method: "POST",
    fields: ["username", "displayName", "password"],
    submit: "createSubmit",
    pending: "createPending",
  },
  profile: {
    path: "/api/me",
    method: "PATCH",
    fields: ["displayName"],
    submit: "profileSubmit",
    pending: "profilePending",
  },
  password: {
    path: "/api/me/password",
    method: "PUT",
    fields: ["currentPassword", "newPassword"],
    submit: "passwordSubmit",
    pending: "passwordPending",
  },
} as const;

type FormKind = keyof typeof forms;
type CreatedAccount = { username: string; displayName: string };

export function AccountForm({ kind, displayName }: { kind: FormKind; displayName?: string }) {
  const t = useTranslations("Accounts");
  const router = useRouter();
  const id = useId();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<CreatedAccount | true | null>(null);
  const message = useProblemMessage(error, "account_unavailable");
  const form = forms[kind];

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const element = event.currentTarget;
    const data = new FormData(element);
    const body: Record<string, string> = {};
    setError(null);
    setSaved(null);

    for (const field of form.fields) {
      const definition = fields[field];
      const value = String(data.get(field) ?? "");
      const length = Array.from(value).length;
      const nameInvalid = definition.type === "text" && (!value.trim() || /\p{Cc}/u.test(value));
      if (length < definition.min || length > definition.max || nameInvalid) {
        setError(definition.error);
        return;
      }
      body[definition.jsonName] = value;
    }

    setBusy(true);
    try {
      const response = await fetch(form.path, {
        method: form.method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!response.ok) {
        setError(await readProblemCode(response) ?? "account_unavailable");
        return;
      }
      if (kind === "create") {
        const created = await response.json();
        setSaved({ username: created.username, displayName: created.display_name });
      } else {
        setSaved(true);
      }
      if (kind === "profile") {
        router.refresh();
      } else {
        element.reset();
      }
    } catch {
      setError("account_unavailable");
    } finally {
      setBusy(false);
    }
  }

  let confirmation: string | null = null;
  if (saved) {
    confirmation = saved === true
      ? t(kind === "profile" ? "profileSaved" : "passwordSaved")
      : t("accountCreated", saved);
  }

  return (
    <form method="post" noValidate onSubmit={save}>
      <fieldset className="grid min-w-0 gap-5" disabled={busy} aria-busy={busy}>
        {form.fields.map((field) => {
          const definition = fields[field];
          const fieldId = `${id}-${field}`;
          const invalid = error === definition.error || ("conflict" in definition && error === definition.conflict);
          const hint = "hint" in definition ? definition.hint : null;
          const descriptions = [hint && `${fieldId}-hint`, invalid && `${id}-error`].filter(Boolean).join(" ");
          return (
            <div key={field} className="grid gap-2">
              <Label htmlFor={fieldId}>{t(field)}</Label>
              <Input
                id={fieldId}
                name={field}
                className="h-11 bg-card shadow-none md:text-base"
                defaultValue={field === "displayName" ? displayName : undefined}
                type={definition.type}
                autoComplete={definition.autoComplete}
                autoCapitalize={field === "username" ? "none" : undefined}
                spellCheck={field === "displayName"}
                aria-describedby={descriptions || undefined}
                aria-invalid={invalid || undefined}
                required
              />
              {hint && <p id={`${fieldId}-hint`} className="text-sm text-muted-foreground">{t(hint)}</p>}
            </div>
          );
        })}
        {message && (
          <p id={`${id}-error`} className="text-sm text-destructive" role="alert">
            {message}
            {error === "session_required" && (
              <> <Link href="/login" className="font-semibold underline underline-offset-2">{t("signInAgain")}</Link></>
            )}
          </p>
        )}
        <Button className="h-11 justify-self-start px-4 text-base font-semibold" type="submit">
          {t(busy ? form.pending : form.submit)}
        </Button>
      </fieldset>
      <p className={confirmation ? "mt-4 text-sm font-medium wrap-anywhere whitespace-pre-wrap text-primary" : undefined} role="status">
        {confirmation}
      </p>
    </form>
  );
}
