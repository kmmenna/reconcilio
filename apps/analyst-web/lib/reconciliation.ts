import type { FinancialAccount, Source } from "@/lib/imports";

export type ReconciliationJob = {
  id: number;
  financial_account_id: string;
  status: "queued" | "running" | "retrying" | "completed" | "failed" | "cancelled";
  attempt: number;
  max_attempts: number;
  completed_at?: string;
  automatic_count?: number;
};

export type Reconciliation = {
  id: string;
  financial_account: FinancialAccount;
  amount: string;
  currency: string;
  confirmed_at: string;
  method: "automatic" | "manual";
};

export type ReconciliationDetail = Reconciliation & {
  actor: { kind: "engine" } | { kind: "analyst"; user_id: string; display_name: string };
  members: Transaction[];
  evidence?: {
    payment_reference: string;
    internal_reference_count: number;
    external_reference_count: number;
    amount: string;
    currency: string;
  };
};

export type Transaction = {
  id: string;
  side: Source["side"];
  source: Source;
  source_reference?: string;
  payment_reference?: string;
  transaction_date: string;
  amount: string;
  currency: string;
  description: string;
};
