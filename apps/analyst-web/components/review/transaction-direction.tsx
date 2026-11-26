import { ArrowDownLeftIcon, ArrowUpRightIcon, MinusIcon } from "lucide-react";
import { getTranslations } from "next-intl/server";

export async function TransactionDirection({ amount }: { amount: string }) {
  const t = await getTranslations("Reconciliation.directions");
  const value = Number(amount);
  const direction = value > 0 ? "inflow" : value < 0 ? "outflow" : "none";
  const Icon = { inflow: ArrowDownLeftIcon, outflow: ArrowUpRightIcon, none: MinusIcon }[direction];

  return (
    <span className="inline-flex items-center gap-1 text-sm font-normal text-muted-foreground">
      <Icon aria-hidden="true" className="size-4 shrink-0" />
      {t(direction)}
    </span>
  );
}
