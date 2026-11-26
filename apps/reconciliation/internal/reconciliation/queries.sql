-- name: ListAutomaticPairs :many
WITH unique_references AS (
    SELECT payment_reference
    FROM transactions
    WHERE financial_account_id = $1 AND payment_reference IS NOT NULL
    GROUP BY payment_reference
    HAVING count(*) FILTER (WHERE source_side = 'internal') = 1
       AND count(*) FILTER (WHERE source_side = 'external') = 1
)
SELECT internal.id AS internal_id, external.id AS external_id, internal.payment_reference,
       internal.amount_minor, internal.currency, internal.minor_digits
FROM unique_references
JOIN transactions AS internal ON internal.financial_account_id = $1
    AND internal.source_side = 'internal' AND internal.payment_reference = unique_references.payment_reference
JOIN transactions AS external ON external.financial_account_id = $1
    AND external.source_side = 'external' AND external.payment_reference = unique_references.payment_reference
WHERE internal.amount_minor = external.amount_minor
  AND internal.currency = external.currency
  AND internal.minor_digits = external.minor_digits
  AND NOT EXISTS (
      SELECT 1 FROM reconciliation_members
      WHERE transaction_id IN (internal.id, external.id) AND released_at IS NULL
  )
ORDER BY internal.id;

-- name: CreateReconciliation :one
INSERT INTO reconciliations (financial_account_id, method, confirmed_by, evidence, job_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: InsertReconciliationPair :exec
INSERT INTO reconciliation_members (reconciliation_id, transaction_id, side)
VALUES (sqlc.arg(reconciliation_id), sqlc.arg(internal_id), 'internal'),
       (sqlc.arg(reconciliation_id), sqlc.arg(external_id), 'external');

-- name: CountAutomaticReconciliations :one
SELECT count(*) FROM reconciliations
WHERE job_id = $1 AND method = 'automatic';

-- name: ListReconciliations :many
SELECT r.id, r.confirmed_at, r.method, sqlc.embed(financial_accounts),
       t.amount_minor, t.currency, t.minor_digits
FROM reconciliations AS r
JOIN financial_accounts ON financial_accounts.id = r.financial_account_id
JOIN reconciliation_members AS m ON m.reconciliation_id = r.id AND m.side = 'internal' AND m.released_at IS NULL
JOIN transactions AS t ON t.id = m.transaction_id
WHERE sqlc.narg('account_id')::uuid IS NULL OR r.financial_account_id = sqlc.narg('account_id')::uuid
ORDER BY r.confirmed_at DESC, r.id;

-- name: GetReconciliation :one
SELECT sqlc.embed(reconciliations), sqlc.embed(financial_accounts),
       coalesce(users.display_name, '') AS analyst_name
FROM reconciliations
JOIN financial_accounts ON financial_accounts.id = reconciliations.financial_account_id
LEFT JOIN users ON users.id = reconciliations.confirmed_by
WHERE reconciliations.id = $1;

-- name: ListReconciliationMembers :many
SELECT sqlc.embed(transactions), reconciliation_members.side, sources.name AS source_name
FROM reconciliation_members
JOIN transactions ON transactions.id = reconciliation_members.transaction_id
JOIN sources ON sources.id = transactions.source_id
WHERE reconciliation_members.reconciliation_id = $1
ORDER BY reconciliation_members.side DESC, transactions.id;
