ALTER TABLE imports
    ADD COLUMN confirmed_at timestamptz,
    ADD COLUMN confirmed_by uuid REFERENCES users (id),
    ADD COLUMN receipt jsonb,
    ADD CHECK (status <> 'confirmed' OR
        (confirmed_at IS NOT NULL AND confirmed_by IS NOT NULL AND receipt IS NOT NULL AND preview IS NULL));

CREATE TABLE transactions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    import_id uuid NOT NULL REFERENCES imports (id),
    financial_account_id uuid NOT NULL REFERENCES financial_accounts (id),
    source_id uuid NOT NULL REFERENCES sources (id),
    source_side text NOT NULL CHECK (source_side IN ('internal', 'external')),
    source_reference text,
    payment_reference text,
    transaction_date date NOT NULL,
    amount_minor bigint NOT NULL,
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    minor_digits integer NOT NULL CHECK (minor_digits BETWEEN 0 AND 4),
    description text NOT NULL
);

CREATE INDEX transactions_account_payment_reference
ON transactions (financial_account_id, payment_reference, source_side) WHERE payment_reference IS NOT NULL;

CREATE TABLE import_coverage (
    import_id uuid NOT NULL REFERENCES imports (id),
    financial_account_id uuid NOT NULL REFERENCES financial_accounts (id),
    source_id uuid NOT NULL REFERENCES sources (id),
    period_start date NOT NULL,
    period_end date NOT NULL,
    all_have_source_references boolean NOT NULL,
    job_id bigint NOT NULL UNIQUE,
    PRIMARY KEY (import_id, financial_account_id),
    CHECK (period_start <= period_end)
);

CREATE TABLE reconciliations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    financial_account_id uuid NOT NULL REFERENCES financial_accounts (id),
    confirmed_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    method text NOT NULL CHECK (method IN ('automatic', 'manual')),
    confirmed_by uuid REFERENCES users (id),
    evidence jsonb,
    job_id bigint,
    CHECK (method <> 'automatic' OR (confirmed_by IS NULL AND evidence IS NOT NULL AND job_id IS NOT NULL)),
    CHECK (method <> 'manual' OR confirmed_by IS NOT NULL)
);

CREATE INDEX reconciliations_account_confirmation
ON reconciliations (financial_account_id, confirmed_at DESC, id);

CREATE INDEX reconciliations_job ON reconciliations (job_id);

CREATE TABLE reconciliation_members (
    reconciliation_id uuid NOT NULL REFERENCES reconciliations (id),
    transaction_id uuid NOT NULL REFERENCES transactions (id),
    side text NOT NULL CHECK (side IN ('internal', 'external')),
    released_at timestamptz,
    PRIMARY KEY (reconciliation_id, transaction_id)
);

CREATE UNIQUE INDEX reconciliation_members_active_transaction
ON reconciliation_members (transaction_id) WHERE released_at IS NULL;
