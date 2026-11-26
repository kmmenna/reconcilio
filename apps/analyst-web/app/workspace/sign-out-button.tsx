"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

export function SignOutButton() {
  const router = useRouter();
  const t = useTranslations("Workspace");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const message = useProblemMessage(error, "sign_out_unavailable");

  async function signOut() {
    setBusy(true);
    setError(null);
    try {
      const response = await fetch("/api/auth/logout", { method: "POST" });
      if (response.ok) {
        router.replace("/login");
        router.refresh();
      } else {
        setError(await readProblemCode(response) ?? "sign_out_unavailable");
      }
    } catch {
      setError("sign_out_unavailable");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-2">
      <Button variant="outline" className="h-11 bg-card px-3 text-base font-semibold desktop:px-4" onClick={signOut} type="button" disabled={busy}>
        {busy ? t("signingOut") : t("signOut")}
      </Button>
      {message && <p className="max-w-44 text-sm text-destructive" role="alert">{message}</p>}
    </div>
  );
}
