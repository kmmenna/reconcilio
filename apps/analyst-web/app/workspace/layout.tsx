import { getTranslations } from "next-intl/server";
import Link from "next/link";
import type { ReactNode } from "react";

import { LanguageControl } from "@/components/language/language-control";
import { getWorkspaceProfile } from "@/lib/workspace";
import { SignOutButton } from "./sign-out-button";

export default async function WorkspaceLayout({ children }: { children: ReactNode }) {
  const [profile, t] = await Promise.all([getWorkspaceProfile(), getTranslations("Workspace")]);

  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex min-h-[4.75rem] flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-border bg-card px-[max(1.3rem,5vw)] py-3">
        <Link href="/workspace" className="wordmark">Reconcilio</Link>
        <div className="flex min-w-0 flex-wrap items-start gap-x-2 gap-y-2 desktop:gap-x-4">
          {profile && <SignOutButton />}
        </div>
      </header>
      {children}
      <footer className="mt-auto flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-t border-border bg-card px-[max(1.3rem,5vw)] py-3">
        <p className="text-sm text-muted-foreground">Reconcilio · {t("footerDescription")}</p>
        <LanguageControl />
      </footer>
    </div>
  );
}
