import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";

export async function AccountLinks({ isAdmin }: { isAdmin: boolean }) {
  const t = await getTranslations("Accounts");
  return (
    <div className="flex flex-wrap gap-3">
      <Button asChild variant="outline" className="h-11 bg-card text-base">
        <Link href="/workspace/profile">{t("profileTitle")}</Link>
      </Button>
      {isAdmin && (
        <Button asChild variant="outline" className="h-11 bg-card text-base">
          <Link href="/workspace/users/new">{t("createTitle")}</Link>
        </Button>
      )}
    </div>
  );
}
