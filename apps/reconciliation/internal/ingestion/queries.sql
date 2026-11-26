-- name: CreatePendingImport :one
INSERT INTO imports (
    source_id, source_side, period_start, period_end, file_fingerprint, preview, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: AssignImportAccounts :exec
INSERT INTO import_financial_accounts (import_id, financial_account_id)
SELECT sqlc.arg(import_id)::uuid, unnest(sqlc.arg(account_ids)::uuid[]);

-- name: ListImportAccounts :many
SELECT a.id, a.code, a.name
FROM import_financial_accounts ia
JOIN financial_accounts a ON a.id = ia.financial_account_id
WHERE ia.import_id = $1
ORDER BY a.id;

-- name: GetImport :one
SELECT imports.id, imports.status, imports.source_id, sources.name AS source_name, imports.source_side,
       imports.period_start, imports.period_end, imports.file_fingerprint, imports.preview, imports.receipt
FROM imports
JOIN sources ON sources.id = imports.source_id
WHERE imports.id = $1;

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

-- name: FindConfirmedReplay :one
SELECT i.id, i.receipt
FROM imports i
WHERE i.source_id = $1 AND i.file_fingerprint = $2 AND i.status = 'confirmed'
  AND ARRAY(
    SELECT ia.financial_account_id
    FROM import_financial_accounts ia
    WHERE ia.import_id = i.id
    ORDER BY ia.financial_account_id
  ) = sqlc.arg(account_ids)::uuid[]
  -- Imports resolved as replays share the original receipt and have no coverage of their own.
  AND EXISTS (SELECT 1 FROM import_coverage c WHERE c.import_id = i.id)
ORDER BY i.confirmed_at, i.id
LIMIT 1;

-- name: ResolveImportReplay :exec
UPDATE imports AS replay
SET status = 'confirmed', preview = NULL, receipt = original.receipt,
    confirmed_at = original.confirmed_at, confirmed_by = original.confirmed_by
FROM imports AS original
WHERE replay.id = sqlc.arg(id) AND original.id = sqlc.arg(original_id);

-- name: ListReferencedTransactions :many
SELECT id, source_reference, payment_reference, transaction_date,
       amount_minor, currency, minor_digits, description
FROM transactions
WHERE source_id = $1 AND financial_account_id = $2 AND source_reference = ANY(sqlc.arg(source_references)::text[]);

-- name: UpdateImportPreview :exec
UPDATE imports SET preview = $2 WHERE id = $1;

-- name: CancelImport :exec
UPDATE imports SET status = 'canceled', preview = NULL, receipt = $2 WHERE id = $1;

-- name: FindBlockedCoverage :one
SELECT import_id, period_start, period_end, all_have_source_references
FROM import_coverage
WHERE source_id = $1 AND financial_account_id = $2
  AND period_end >= sqlc.arg(incoming_start)::date
  AND period_start <= sqlc.arg(incoming_end)::date
  AND (NOT all_have_source_references OR NOT sqlc.arg(incoming_has_source_references)::boolean)
ORDER BY period_start, period_end, import_id
LIMIT 1;
