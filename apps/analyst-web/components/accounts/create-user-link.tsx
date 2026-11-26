import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";

export async function CreateUserLink() {
  const t = await getTranslations("Accounts");
  return (
    <Button asChild variant="outline" className="h-11 border-panel-divider bg-transparent px-3 text-panel-foreground hover:border-panel-muted hover:bg-transparent hover:text-panel-foreground focus-visible:outline-panel-muted">
      <Link href="/workspace/users/new">{t("createTitle")}</Link>
    </Button>
  );
}
