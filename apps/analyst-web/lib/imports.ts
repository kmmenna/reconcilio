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

export type ImportPreview = {
  id: string;
  status: "pending";
  source: Source;
  financial_account: FinancialAccount;
  period_start: string;
  period_end: string;
  rows: PreviewRow[];
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
