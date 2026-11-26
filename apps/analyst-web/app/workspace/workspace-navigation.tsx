"use client";

import { ArrowLeftRightIcon, BookOpenIcon, ClipboardListIcon, FileUpIcon, ListChecksIcon, MenuIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { useSelectedLayoutSegment } from "next/navigation";
import { useId, useRef, useState } from "react";

const destinations = [
  { segment: "imports", message: "Imports.title", Icon: FileUpIcon },
  { segment: "reconciliations", message: "Reconciliation.title", Icon: ListChecksIcon },
  { segment: "review", message: "Review.title", Icon: ClipboardListIcon },
  { segment: "transactions", message: "Transactions.title", Icon: ArrowLeftRightIcon },
  { segment: "catalog", message: "Catalog.title", Icon: BookOpenIcon },
] as const;

export function WorkspaceNavigation() {
  const t = useTranslations();
  const segment = useSelectedLayoutSegment();
  const activeSection = segment === "match-proposals" ? "review" : segment;
  const [menuOpen, setMenuOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const linksID = useId();

  function closeMenu() {
    if (!menuOpen) return;
    setMenuOpen(false);
    menuButton.current?.focus();
  }

  return (
    <nav
      aria-label={t("Workspace.navigation")}
      className="border-b border-sidebar-divider bg-sidebar text-sidebar-foreground lg:border-r lg:border-b-0"
      onKeyDown={(event) => {
        if (event.key === "Escape") closeMenu();
      }}
    >
      <button
        ref={menuButton}
        type="button"
        aria-expanded={menuOpen}
        aria-controls={linksID}
        onClick={() => setMenuOpen(!menuOpen)}
        className="mx-[1.3rem] my-1 flex min-h-11 cursor-pointer items-center gap-2 rounded-md px-2 text-sm font-semibold hover:bg-sidebar-hover focus-visible:outline-primary lg:hidden"
      >
        <MenuIcon aria-hidden="true" className="size-[1.125rem]" />
        {t("Workspace.menu")}
      </button>
      <ul
        id={linksID}
        className={`${menuOpen ? "grid" : "hidden"} gap-1 px-[1.3rem] pb-3 lg:sticky lg:top-0 lg:grid lg:max-h-svh lg:overflow-y-auto lg:px-4 lg:py-8`}
      >
        {destinations.map(({ segment, message, Icon }) => {
          const current = segment === activeSection;
          return (
            <li key={segment}>
              <Link
                href={`/workspace/${segment}`}
                aria-current={current ? "location" : undefined}
                onNavigate={closeMenu}
                className="flex min-h-11 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium
                  hover:bg-sidebar-hover focus-visible:-outline-offset-2 focus-visible:outline-primary
                  aria-[current=location]:bg-sidebar-active aria-[current=location]:font-semibold
                  aria-[current=location]:text-sidebar-active-foreground
                  aria-[current=location]:focus-visible:outline-sidebar-active-foreground"
              >
                <Icon aria-hidden="true" className="size-[1.125rem] shrink-0" />
                {t(message)}
                {current && <span aria-hidden="true" className="ms-auto size-[5px] shrink-0 rounded-full bg-current" />}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
