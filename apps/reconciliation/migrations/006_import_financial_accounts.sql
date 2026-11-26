CREATE TABLE import_financial_accounts (
    import_id uuid NOT NULL REFERENCES imports (id),
    financial_account_id uuid NOT NULL REFERENCES financial_accounts (id),
    PRIMARY KEY (import_id, financial_account_id)
);

INSERT INTO import_financial_accounts (import_id, financial_account_id)
SELECT id, financial_account_id FROM imports;

ALTER TABLE transactions ADD FOREIGN KEY (import_id, financial_account_id)
REFERENCES import_financial_accounts (import_id, financial_account_id);

ALTER TABLE import_coverage ADD FOREIGN KEY (import_id, financial_account_id)
REFERENCES import_financial_accounts (import_id, financial_account_id);

DROP INDEX imports_confirmed_fingerprint;
ALTER TABLE imports DROP COLUMN financial_account_id;
CREATE INDEX imports_confirmed_fingerprint
ON imports (source_id, file_fingerprint) WHERE status = 'confirmed';
