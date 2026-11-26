"use client";

import { ChevronDownIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { DropdownMenu } from "radix-ui";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { correlatedFetch } from "@/lib/correlation";
import { readProblemCode, useProblemMessage } from "@/lib/problem-message";

const itemClassName = "flex min-h-11 cursor-pointer items-center rounded-md px-3 py-2 text-sm font-medium data-highlighted:bg-accent focus-visible:-outline-offset-2 data-disabled:cursor-wait data-disabled:opacity-60";

export function AccountMenu({ displayName, username }: { displayName: string; username: string }) {
  const router = useRouter();
  const t = useTranslations("Workspace");
  const accounts = useTranslations("Accounts");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const message = useProblemMessage(error, "sign_out_unavailable");

  async function signOut() {
    setBusy(true);
    setError(null);
    try {
      const response = await correlatedFetch("/api/auth/logout", { method: "POST" });
      if (response.ok) {
        router.replace("/login");
        router.refresh();
      } else {
        setError(await readProblemCode(response) ?? "sign_out_unavailable");
      }
    } catch {
      setError("sign_out_unavailable");
    } finally {
      setBusy(false);
    }
  }

  return (
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild>
        <Button variant="ghost" className="h-auto min-h-11 max-w-[min(16rem,100%)] px-3 py-1.5 text-start hover:bg-panel-hover hover:text-panel-foreground focus-visible:outline-panel-muted aria-expanded:bg-panel-hover">
          <span className="min-w-0">
            <bdi className="block overflow-hidden text-ellipsis whitespace-pre font-semibold" title={displayName}>{displayName}</bdi>
            <bdi className="block overflow-hidden text-ellipsis whitespace-pre text-xs text-panel-muted" title={username}>{username}</bdi>
          </span>
          <ChevronDownIcon aria-hidden="true" className="size-3.5 text-panel-muted" />
        </Button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={6}
          collisionPadding={16}
          className="z-50 w-48 max-h-(--radix-dropdown-menu-content-available-height) max-w-(--radix-dropdown-menu-content-available-width) overflow-y-auto rounded-md border border-border bg-card p-1 text-foreground outline-none"
        >
          <DropdownMenu.Item asChild className={itemClassName}>
            <Link href="/workspace/profile">{accounts("profileTitle")}</Link>
          </DropdownMenu.Item>
          <DropdownMenu.Item
            className={itemClassName}
            disabled={busy}
            onSelect={(event) => {
              event.preventDefault();
              void signOut();
            }}
          >
            {busy ? t("signingOut") : t("signOut")}
          </DropdownMenu.Item>
          {message && <p className="px-3 py-2 text-sm text-destructive" role="alert">{message}</p>}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
