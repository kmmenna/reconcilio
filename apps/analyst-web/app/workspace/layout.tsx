import { getTranslations } from "next-intl/server";
import Link from "next/link";
import type { ReactNode } from "react";

import { CreateUserLink } from "@/components/accounts/create-user-link";
import { LanguageControl } from "@/components/language/language-control";
import { getWorkspaceProfile } from "@/lib/workspace";
import { AccountMenu } from "./account-menu";
import { WorkspaceNavigation } from "./workspace-navigation";

export default async function WorkspaceLayout({ children }: { children: ReactNode }) {
  const [profile, t] = await Promise.all([getWorkspaceProfile(), getTranslations("Workspace")]);

  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex min-h-[4.75rem] flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-panel bg-panel px-[max(1.3rem,5vw)] py-3 text-panel-foreground">
        <Link href="/workspace" className="wordmark focus-visible:outline-panel-muted">Reconcilio</Link>
        <div className="ml-auto flex min-w-0 max-w-full flex-wrap items-center justify-end gap-x-2 gap-y-2 desktop:gap-x-4">
          {profile?.is_admin && <CreateUserLink />}
          {profile && <AccountMenu displayName={profile.display_name} username={profile.username} />}
        </div>
      </header>
      <div className={profile ? "flex flex-1 flex-col lg:grid lg:grid-cols-[15rem_minmax(0,1fr)]" : "flex-1"}>
        {profile && <WorkspaceNavigation />}
        {children}
      </div>
      <footer className="mt-auto flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-t border-border bg-card px-[max(1.3rem,5vw)] py-3">
        <p className="text-sm text-muted-foreground">Reconcilio · {t("footerDescription")}</p>
        <LanguageControl side="top" />
      </footer>
    </div>
  );
}
