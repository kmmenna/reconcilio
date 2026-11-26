-- name: ListFinancialAccounts :many
SELECT id, code, name
FROM financial_accounts
ORDER BY name, id;

-- name: ListSources :many
SELECT id, name, side, NOT EXISTS (
    SELECT 1 FROM imports WHERE source_id = sources.id AND status = 'confirmed'
) AS side_editable
FROM sources
ORDER BY name, id;

-- name: GetFinancialAccount :one
SELECT id, code, name
FROM financial_accounts
WHERE id = $1;

-- name: GetSource :one
SELECT id, name, side
FROM sources
WHERE id = $1;

-- name: LockFinancialAccounts :many
SELECT id
FROM financial_accounts
WHERE id = ANY($1::uuid[])
ORDER BY id
FOR NO KEY UPDATE;

-- name: CreateFinancialAccount :one
INSERT INTO financial_accounts (code, name) VALUES ($1, $2)
RETURNING id, code, name;

-- name: RenameFinancialAccount :one
UPDATE financial_accounts SET name = $2 WHERE id = $1
RETURNING id, code, name;

-- name: CreateSource :one
INSERT INTO sources (name, side) VALUES ($1, $2)
RETURNING id, name, side;

-- name: LockSourceForEdit :one
SELECT id, name, side FROM sources WHERE id = $1 FOR NO KEY UPDATE;

-- name: LockSourceForImport :one
SELECT id, name, side FROM sources WHERE id = $1 FOR SHARE;

-- name: SourceHasConfirmedImport :one
SELECT EXISTS (SELECT 1 FROM imports WHERE source_id = $1 AND status = 'confirmed');

-- name: UpdateSource :one
UPDATE sources SET name = $2, side = $3 WHERE id = $1
RETURNING id, name, side;
