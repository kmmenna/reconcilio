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
  actor: DecisionActor;
  match_proposal_id?: string;
  proposal_evidence?: ProposalEvidence;
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

export type OpenTransaction = Transaction & {
  financial_account: FinancialAccount;
  has_pending_match_proposals: boolean;
  amount_discrepancy?: { payment_reference: string; amount: string; currency: string };
};

export type DecisionActor = { kind: "engine" } | { kind: "analyst"; user_id: string; display_name: string };

export type ProposalEvidence = {
  reference_relation: "missing" | "different" | "shared";
  days_apart: number;
  internal_reference_count?: number;
  external_reference_count?: number;
  amount: string;
  currency: string;
};

export type ProposalResolution = {
  status: "pending" | "accepted" | "rejected" | "invalidated";
  resolved_at?: string;
  actor?: DecisionActor;
  reconciliation_id?: string;
};

export type MatchProposal = ProposalResolution & {
  id: string;
  external_transaction: Transaction;
  evidence: ProposalEvidence;
  created_at: string;
};

export type ReviewTransaction = {
  financial_account: FinancialAccount;
  internal_transaction: Transaction;
};
export type ReviewQueueEntry = ReviewTransaction & { pending_count: number };
export type ReviewGroup = ReviewTransaction & { proposals: MatchProposal[] };
export type MatchProposalDetail = ReviewTransaction & MatchProposal;
