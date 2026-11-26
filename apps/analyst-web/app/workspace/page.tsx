import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { readProfile, sessionCookie } from "@/lib/auth";
import { SignOutButton } from "./sign-out-button";

export default async function WorkspacePage() {
  const token = (await cookies()).get(sessionCookie)?.value;
  if (!token) redirect("/login");

  const profile = await readProfile(token).catch(() => undefined);
  if (profile === null) redirect("/login");
  if (!profile) {
    return (
      <main className="workspace">
        <h1>Workspace unavailable</h1>
        <p role="alert">Your session could not be verified. Reload the page to try again.</p>
      </main>
    );
  }

  return (
    <>
      <header className="workspace-header">
        <span className="wordmark">Reconcilio</span>
        <SignOutButton />
      </header>
      <main className="workspace">
        <h1>Reconciliation workspace</h1>
        <dl className="session-details">
          <div><dt>Signed in as</dt><dd>{profile.display_name}</dd></div>
          <div><dt>Username</dt><dd>{profile.username}</dd></div>
        </dl>
      </main>
    </>
  );
}
