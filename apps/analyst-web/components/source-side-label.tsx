import { LogInIcon, LogOutIcon } from "lucide-react";
import type { ReactNode } from "react";

import type { Source } from "@/lib/imports";

export function SourceSideLabel({ side, children }: { side: Source["side"]; children: ReactNode }) {
  const Icon = side === "internal" ? LogInIcon : LogOutIcon;
  return (
    <span className="inline-flex max-w-full items-center gap-1.5 align-middle">
      <Icon aria-hidden="true" className="size-4 shrink-0" />
      <span className="min-w-0 wrap-anywhere">{children}</span>
    </span>
  );
}
