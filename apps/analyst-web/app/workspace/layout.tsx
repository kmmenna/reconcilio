import Link from "next/link";
import type { ReactNode } from "react";

import { LanguageControl } from "@/components/language/language-control";
import { getWorkspaceProfile } from "@/lib/workspace";
import { AccountMenu } from "./account-menu";

export default async function WorkspaceLayout({ children }: { children: ReactNode }) {
  const profile = await getWorkspaceProfile();

  return (
    <>
      <header className="flex min-h-[4.75rem] flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-border bg-card px-[max(1.3rem,5vw)] py-3">
        <Link href="/workspace" className="wordmark">Reconcilio</Link>
        <div className="ml-auto flex min-w-0 max-w-full flex-wrap items-start justify-end gap-x-2 gap-y-2 desktop:gap-x-4">
          <LanguageControl />
          {profile && <AccountMenu displayName={profile.display_name} username={profile.username} />}
        </div>
      </header>
      {children}
    </>
  );
}
