import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { getTranslations } from "next-intl/server";
import type { ReactNode } from "react";

export async function AccountPage({ title, children }: { title: string; children: ReactNode }) {
  const t = await getTranslations("Accounts");
  return (
    <main className="workspace">
      <Link
        href="/workspace"
        className="inline-flex min-h-9 items-center gap-2 rounded-md text-sm font-medium text-primary underline-offset-4 hover:underline"
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {t("workspace")}
      </Link>
      <h1 className="page-title mt-3">{title}</h1>
      <div className="mt-8 max-w-4xl border-b border-border">{children}</div>
    </main>
  );
}

export function AccountSetting({
  title,
  children,
  description,
}: {
  title: string;
  children: ReactNode;
  description?: ReactNode;
}) {
  return (
    <section className="grid gap-5 border-t border-border py-6 desktop:grid-cols-[minmax(12rem,1fr)_minmax(16rem,1.4fr)] desktop:gap-10 desktop:py-8">
      <div>
        <h2 className="text-lg leading-snug font-semibold">{title}</h2>
        {description && <div className="mt-2 max-w-[34ch] text-sm leading-relaxed text-muted-foreground">{description}</div>}
      </div>
      <div className="min-w-0 max-w-md">{children}</div>
    </section>
  );
}
