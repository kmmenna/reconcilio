CREATE TABLE ranking_evaluations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    internal_transaction_id uuid NOT NULL REFERENCES transactions (id),
    created_at timestamptz NOT NULL DEFAULT statement_timestamp()
);

CREATE INDEX ranking_evaluations_group
ON ranking_evaluations (internal_transaction_id, created_at DESC, id DESC);

CREATE TABLE ranking_assessments (
    evaluation_id uuid NOT NULL REFERENCES ranking_evaluations (id),
    proposal_id uuid NOT NULL REFERENCES match_proposals (id),
    rank integer NOT NULL CHECK (rank > 0),
    score integer NOT NULL CHECK (score BETWEEN 0 AND 100),
    explanation_pt_br text NOT NULL CHECK (length(explanation_pt_br) BETWEEN 1 AND 400),
    explanation_en text NOT NULL CHECK (length(explanation_en) BETWEEN 1 AND 400),
    PRIMARY KEY (evaluation_id, proposal_id),
    UNIQUE (evaluation_id, rank)
);

ALTER TABLE match_proposals
ADD COLUMN ranking_evaluation_id uuid,
ADD FOREIGN KEY (ranking_evaluation_id, id) REFERENCES ranking_assessments (evaluation_id, proposal_id),
ADD CHECK (ranking_evaluation_id IS NULL OR status IN ('accepted', 'rejected'));

CREATE TABLE ranking_daily_usage (
    usage_day date PRIMARY KEY,
    call_count integer NOT NULL CHECK (call_count > 0)
);

CREATE TABLE ranking_job_usage (
    job_id bigint PRIMARY KEY,
    call_count integer NOT NULL CHECK (call_count > 0)
);
