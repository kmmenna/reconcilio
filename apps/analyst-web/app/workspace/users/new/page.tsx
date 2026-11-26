import { getTranslations } from "next-intl/server";

import { AccountForm } from "@/components/accounts/account-form";
import { AccountPage, AccountSetting } from "@/components/accounts/account-page";
import { getWorkspaceProfile } from "@/lib/workspace";

export default async function NewUserPage() {
  const [profile, t, errors] = await Promise.all([
    getWorkspaceProfile(),
    getTranslations("Accounts"),
    getTranslations("Errors"),
  ]);

  return (
    <AccountPage title={t("createTitle")}>
      {profile?.is_admin ? (
        <AccountSetting title={t("newAccountHeading")} description={t("createDescription")}>
          <AccountForm kind="create" />
        </AccountSetting>
      ) : (
        <p className="border-t border-border py-6 text-destructive" role="alert">
          {profile ? errors("administrator_required") : t("unavailable")}
        </p>
      )}
    </AccountPage>
  );
}
