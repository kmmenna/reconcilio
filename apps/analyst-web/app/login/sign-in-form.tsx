"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

export function SignInForm() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setBusy(true);
    const fields = new FormData(event.currentTarget);
    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: fields.get("username"), password: fields.get("password") }),
      });
      if (response.ok) {
        router.replace("/workspace");
        router.refresh();
        return;
      }
      const problem = await response.json();
      setError(problem.detail ?? "Sign-in failed. Try again.");
    } catch {
      setError("Sign-in is temporarily unavailable. Try again in a moment.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="sign-in-form" method="post" onSubmit={signIn}>
      <label>
        Username
        <input name="username" autoComplete="username" required />
      </label>
      <label>
        Password
        <input name="password" type="password" autoComplete="current-password" required />
      </label>
      {error && <p className="form-error" role="alert">{error}</p>}
      <button className="primary-button" type="submit" disabled={busy}>
        {busy ? "Signing in…" : "Sign in"}
      </button>
    </form>
  );
}
