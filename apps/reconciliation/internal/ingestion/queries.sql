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
       imports.period_start, imports.period_end, imports.preview
FROM imports
JOIN sources ON sources.id = imports.source_id
JOIN financial_accounts ON financial_accounts.id = imports.financial_account_id
WHERE imports.id = $1;
