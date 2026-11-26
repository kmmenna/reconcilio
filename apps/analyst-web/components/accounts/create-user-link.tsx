import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";

export async function CreateUserLink() {
  const t = await getTranslations("Accounts");
  return (
    <Button asChild variant="outline" className="h-11 px-3">
      <Link href="/workspace/users/new">{t("createTitle")}</Link>
    </Button>
  );
}
