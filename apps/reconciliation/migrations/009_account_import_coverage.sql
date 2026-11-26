-- Existing assignments inherit the recorded declared period from their import.
ALTER TABLE import_financial_accounts
    ADD COLUMN period_start date,
    ADD COLUMN period_end date,
    ADD COLUMN coverage_basis text NOT NULL DEFAULT 'declared' CHECK (coverage_basis IN ('declared', 'inferred')),
    ADD CHECK (
        (period_start IS NULL AND period_end IS NULL AND coverage_basis = 'declared')
        OR (period_start IS NOT NULL AND period_end IS NOT NULL AND period_start <= period_end)
    );
