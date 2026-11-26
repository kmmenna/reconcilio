"use client";

import { correlatedFetch } from "@/lib/correlation";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

export function SignInForm() {
  const router = useRouter();
  const t = useTranslations("SignIn");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const message = useProblemMessage(error, "sign_in_unavailable");

  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setBusy(true);
    const fields = new FormData(event.currentTarget);
    try {
      const response = await correlatedFetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: fields.get("username"), password: fields.get("password") }),
      });
      if (response.ok) {
        router.replace("/workspace");
        router.refresh();
        return;
      }
      setError(await readProblemCode(response) ?? "sign_in_unavailable");
    } catch {
      setError("sign_in_unavailable");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="mt-8 grid gap-5" method="post" noValidate onSubmit={signIn}>
      <div className="grid gap-2">
        <Label htmlFor="username" className="text-[0.95rem] leading-normal font-semibold">{t("username")}</Label>
        <Input id="username" name="username" autoComplete="username" className="h-12 md:text-base" required />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="password" className="text-[0.95rem] leading-normal font-semibold">{t("password")}</Label>
        <Input id="password" name="password" type="password" autoComplete="current-password" className="h-12 md:text-base" required />
      </div>
      {message && <p className="text-sm text-destructive" role="alert">{message}</p>}
      <Button className="h-12 text-base font-semibold" type="submit" disabled={busy}>
        {busy ? t("pending") : t("submit")}
      </Button>
    </form>
  );
}
