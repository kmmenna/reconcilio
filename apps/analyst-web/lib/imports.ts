export type Source = {
  id: string;
  name: string;
  side: "internal" | "external";
};

export type FinancialAccount = {
  id: string;
  code: string;
  name: string;
};

export type PreviewRow = {
  line: number;
  source_reference?: string;
  payment_reference?: string;
  transaction_date: string;
  amount: string;
  currency: string;
  description: string;
};

type ImportSummary = {
  id: string;
  source: Source;
  financial_account: FinancialAccount;
  period_start: string;
  period_end: string;
};

export type ImportPreview = ImportSummary & {
  status: "pending";
  rows: PreviewRow[];
};

export type ImportReceipt = ImportSummary & {
  status: "confirmed";
  confirmed_at: string;
  confirmed_by: string;
  transaction_count: number;
  jobs: { financial_account: FinancialAccount; job_id: number }[];
};

export type ImportIssue = {
  line?: number;
  field: string;
  reason: string;
};

export type ImportProblem = {
  code: string;
  issues?: ImportIssue[];
};

export const csvColumns = [
  "source_reference", "payment_reference", "transaction_date", "amount", "currency", "description",
];
