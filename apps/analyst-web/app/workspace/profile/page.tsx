import { getTranslations } from "next-intl/server";

import { AccountForm } from "@/components/accounts/account-form";
import { AccountPage, AccountSetting } from "@/components/accounts/account-page";
import { getWorkspaceProfile } from "@/lib/workspace";

export default async function ProfilePage() {
  const [profile, t] = await Promise.all([
    getWorkspaceProfile(),
    getTranslations("Accounts"),
  ]);

  return (
    <AccountPage title={t("profileTitle")}>
      {profile ? (
        <>
          <AccountSetting
            title={t("nameHeading")}
            description={(
              <dl>
                <dt>{t("username")}</dt>
                <dd className="font-semibold wrap-anywhere whitespace-pre-wrap text-foreground">{profile.username}</dd>
              </dl>
            )}
          >
            <AccountForm kind="profile" displayName={profile.display_name} />
          </AccountSetting>
          <AccountSetting title={t("passwordHeading")} description={t("passwordDescription")}>
            <AccountForm kind="password" />
          </AccountSetting>
        </>
      ) : (
        <p className="border-t border-border py-6 text-destructive" role="alert">{t("unavailable")}</p>
      )}
    </AccountPage>
  );
}
