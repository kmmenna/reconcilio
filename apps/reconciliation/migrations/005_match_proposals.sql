CREATE TABLE match_proposals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    financial_account_id uuid NOT NULL REFERENCES financial_accounts (id),
    internal_transaction_id uuid NOT NULL REFERENCES transactions (id),
    external_transaction_id uuid NOT NULL REFERENCES transactions (id),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'invalidated')),
    evidence jsonb NOT NULL CHECK (jsonb_typeof(evidence) = 'object'),
    created_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    job_id bigint NOT NULL,
    resolved_at timestamptz,
    resolved_by uuid REFERENCES users (id),
    resolved_by_name text,
    reconciliation_id uuid REFERENCES reconciliations (id),
    CHECK (internal_transaction_id <> external_transaction_id),
    CHECK ((resolved_by IS NULL) = (resolved_by_name IS NULL)),
    CHECK (
        (status = 'pending' AND resolved_at IS NULL AND resolved_by IS NULL AND reconciliation_id IS NULL)
        OR (status = 'accepted' AND resolved_at IS NOT NULL AND resolved_by IS NOT NULL AND reconciliation_id IS NOT NULL)
        OR (status = 'rejected' AND resolved_at IS NOT NULL AND resolved_by IS NOT NULL AND reconciliation_id IS NULL)
        OR (status = 'invalidated' AND resolved_at IS NOT NULL AND reconciliation_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX match_proposals_pair_pending
ON match_proposals (internal_transaction_id, external_transaction_id) WHERE status = 'pending';

CREATE INDEX match_proposals_pair_rejected
ON match_proposals (internal_transaction_id, external_transaction_id) WHERE status = 'rejected';

CREATE INDEX match_proposals_external_pending
ON match_proposals (external_transaction_id) WHERE status = 'pending';

CREATE UNIQUE INDEX match_proposals_accepted_reconciliation
ON match_proposals (reconciliation_id) WHERE status = 'accepted';

CREATE INDEX match_proposals_job ON match_proposals (job_id);

CREATE INDEX transactions_review_eligibility
ON transactions (financial_account_id, source_side, currency, amount_minor, transaction_date);
