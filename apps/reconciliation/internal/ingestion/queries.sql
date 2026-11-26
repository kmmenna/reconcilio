-- name: CreatePendingImport :one
INSERT INTO imports (
    source_id, source_side, financial_account_id, period_start, period_end,
    file_fingerprint, preview, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: GetImport :one
SELECT imports.id, imports.status, imports.source_id, sources.name AS source_name, imports.source_side,
       imports.financial_account_id, financial_accounts.code AS financial_account_code,
       financial_accounts.name AS financial_account_name,
       imports.period_start, imports.period_end, imports.preview, imports.receipt
FROM imports
JOIN sources ON sources.id = imports.source_id
JOIN financial_accounts ON financial_accounts.id = imports.financial_account_id
WHERE imports.id = $1;

-- name: GetImportAccountID :one
SELECT financial_account_id
FROM imports
WHERE id = $1;

-- name: GetImportForConfirmation :one
SELECT imports.id, imports.status, imports.source_id, sources.name AS source_name, imports.source_side,
       sources.side AS current_source_side, imports.financial_account_id,
       financial_accounts.code AS financial_account_code, financial_accounts.name AS financial_account_name,
       imports.period_start, imports.period_end, imports.preview, imports.receipt
FROM imports
JOIN sources ON sources.id = imports.source_id
JOIN financial_accounts ON financial_accounts.id = imports.financial_account_id
WHERE imports.id = $1
FOR SHARE OF sources;

-- name: InsertTransactions :copyfrom
INSERT INTO transactions (
    import_id, financial_account_id, source_id, source_side, source_reference, payment_reference,
    transaction_date, amount_minor, currency, minor_digits, description
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: InsertImportCoverage :exec
INSERT INTO import_coverage (
    import_id, financial_account_id, source_id, period_start, period_end, all_have_source_references, job_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ConfirmImport :exec
UPDATE imports
SET status = 'confirmed', preview = NULL, confirmed_at = $2, confirmed_by = $3, receipt = $4
WHERE id = $1;
