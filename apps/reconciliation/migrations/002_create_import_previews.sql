CREATE TABLE financial_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code text NOT NULL UNIQUE,
    name text NOT NULL
);

CREATE TABLE sources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    side text NOT NULL CHECK (side IN ('internal', 'external'))
);

INSERT INTO financial_accounts (code, name)
VALUES ('corporate-card', 'Corporate Card');

INSERT INTO sources (name, side)
VALUES ('Demo ERP', 'internal'), ('Demo Card Statement', 'external');

CREATE TABLE imports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL DEFAULT 'pending',
    source_id uuid NOT NULL REFERENCES sources (id),
    source_side text NOT NULL CHECK (source_side IN ('internal', 'external')),
    financial_account_id uuid NOT NULL REFERENCES financial_accounts (id),
    period_start date NOT NULL,
    period_end date NOT NULL,
    file_fingerprint bytea NOT NULL,
    preview jsonb,
    created_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (period_start <= period_end),
    CHECK (status <> 'pending' OR preview IS NOT NULL)
);
