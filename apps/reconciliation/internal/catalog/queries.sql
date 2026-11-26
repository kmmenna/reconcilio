-- name: ListFinancialAccounts :many
SELECT id, code, name
FROM financial_accounts
ORDER BY name, id;

-- name: ListSources :many
SELECT id, name, side
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
