CREATE TABLE reconciliation_account_leases (
    financial_account_id uuid PRIMARY KEY REFERENCES financial_accounts (id),
    attempt_token uuid NOT NULL,
    expires_at timestamptz NOT NULL
);

-- This history and its creating job IDs survive River's cleanup.
CREATE TABLE reconciliation_job_summaries (
    job_id bigint PRIMARY KEY,
    financial_account_id uuid NOT NULL REFERENCES financial_accounts (id),
    status text NOT NULL CHECK (status IN ('completed', 'failed', 'cancelled')),
    completed_at timestamptz NOT NULL,
    attempt integer NOT NULL CHECK (attempt >= 0),
    max_attempts integer NOT NULL CHECK (max_attempts > 0),
    automatic_count bigint NOT NULL CHECK (automatic_count >= 0),
    proposal_count bigint NOT NULL CHECK (proposal_count >= 0)
);
