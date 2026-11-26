import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";
import { getWorkspaceProfile } from "@/lib/workspace";

export default async function WorkspacePage() {
  const profile = await getWorkspaceProfile();
  const t = await getTranslations("Workspace");
  const imports = await getTranslations("Imports");

  return (
    <main className="workspace">
      <h1 className="page-title">{profile ? t("title") : t("unavailable")}</h1>
      {profile ? (
        <>
          <dl className="mt-8 grid max-w-xl grid-cols-2 gap-5 border-y border-border py-5">
            <div className="min-w-0">
              <dt className="text-sm text-muted-foreground">{t("signedInAs")}</dt>
              <dd className="mt-1 font-semibold wrap-anywhere">{profile.display_name}</dd>
            </div>
            <div className="min-w-0">
              <dt className="text-sm text-muted-foreground">{t("username")}</dt>
              <dd className="mt-1 font-semibold wrap-anywhere">{profile.username}</dd>
            </div>
          </dl>
          <Button asChild className="mt-6">
            <Link href="/workspace/imports">{imports("title")}</Link>
          </Button>
        </>
      ) : (
        <p className="mt-6 text-destructive" role="alert">{t("sessionUnavailable")}</p>
      )}
    </main>
  );
}
