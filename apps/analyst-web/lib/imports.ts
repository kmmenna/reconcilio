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

export type ConflictField = "amount" | "currency" | "transaction_date" | "payment_reference";

export type TransactionValues = Omit<PreviewRow, "line"> & {
  transaction_id?: string;
};

export type ImportConflict = {
  id: string;
  financial_account_id: string;
  line: number;
  fields: ConflictField[];
  stored: TransactionValues;
  incoming: TransactionValues;
};

export type CoverageOverlap = {
  import_id: string;
  financial_account: FinancialAccount;
  period_start: string;
  period_end: string;
  previous_has_source_references: boolean;
  incoming_has_source_references: boolean;
};

type ImportSummary = {
  id: string;
  source: Source;
  financial_account: FinancialAccount;
  period_start: string;
  period_end: string;
  conflicts?: ImportConflict[];
};

export type ImportPreview = ImportSummary & {
  status: "pending";
  rows: PreviewRow[];
};

export type ImportReceipt = ImportSummary & {
  status: "confirmed";
  decision?: "keep_original";
  confirmed_at: string;
  confirmed_by: string;
  transaction_count: number;
  jobs: { financial_account: FinancialAccount; job_id: number }[];
};

export type ImportCancellation = ImportSummary & {
  status: "canceled";
  decision: "cancel";
  canceled_at: string;
  canceled_by: string;
};

export type ImportView = ImportPreview | ImportReceipt | ImportCancellation;

export type ImportIssue = {
  line?: number;
  field: string;
  reason: string;
};

export type ImportProblem = {
  code: string;
  issues?: ImportIssue[];
  context?: { coverage?: CoverageOverlap; preview?: ImportPreview };
};

export const csvColumns = [
  "source_reference", "payment_reference", "transaction_date", "amount", "currency", "description",
];
