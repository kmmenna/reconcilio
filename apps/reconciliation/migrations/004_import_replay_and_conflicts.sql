CREATE UNIQUE INDEX transactions_source_account_reference
ON transactions (source_id, financial_account_id, source_reference) WHERE source_reference IS NOT NULL;

CREATE INDEX imports_confirmed_fingerprint
ON imports (source_id, financial_account_id, file_fingerprint) WHERE status = 'confirmed';

ALTER TABLE imports ADD CONSTRAINT imports_canceled_receipt
CHECK (status <> 'canceled' OR (receipt IS NOT NULL AND preview IS NULL));

CREATE INDEX import_coverage_source_account_period
ON import_coverage (source_id, financial_account_id, period_start, period_end);
