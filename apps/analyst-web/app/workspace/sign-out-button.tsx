"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

export function SignOutButton() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function signOut() {
    setBusy(true);
    setError("");
    try {
      const response = await fetch("/api/auth/logout", { method: "POST" });
      if (!response.ok) throw new Error("Sign-out failed");
      router.replace("/login");
      router.refresh();
    } catch {
      setError("Could not sign out. Try again.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="sign-out">
      {error && <span className="form-error" role="alert">{error}</span>}
      <button className="secondary-button" onClick={signOut} type="button" disabled={busy}>
        {busy ? "Signing out…" : "Sign out"}
      </button>
    </div>
  );
}
