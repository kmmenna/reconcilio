import { getTranslations } from "next-intl/server";
import { redirect } from "next/navigation";

import { getWorkspaceProfile } from "@/lib/workspace";

export default async function WorkspacePage() {
  if (await getWorkspaceProfile()) redirect("/workspace/imports");

  const t = await getTranslations("Workspace");
  return (
    <main className="workspace">
      <h1 className="page-title">{t("unavailable")}</h1>
      <p className="mt-6 text-destructive" role="alert">{t("sessionUnavailable")}</p>
    </main>
  );
}
