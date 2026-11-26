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
  AND NOT EXISTS (
      SELECT 1 FROM match_proposals
      WHERE internal_transaction_id = internal.id AND external_transaction_id = external.id AND status = 'rejected'
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

-- name: ListReviewCandidates :many
WITH reference_counts AS (
    SELECT payment_reference, count(*) FILTER (WHERE source_side = 'internal') AS internal_count,
           count(*) FILTER (WHERE source_side = 'external') AS external_count
    FROM transactions
    WHERE financial_account_id = sqlc.arg(account_id) AND payment_reference IS NOT NULL
    GROUP BY payment_reference
)
SELECT internal.id AS internal_id, external.id AS external_id,
       CASE
           WHEN internal.payment_reference IS NULL OR external.payment_reference IS NULL THEN 'missing'
           WHEN internal.payment_reference = external.payment_reference THEN 'shared'
           ELSE 'different'
       END AS reference_relation,
       abs(internal.transaction_date - external.transaction_date)::integer AS days_apart,
       coalesce(reference_counts.internal_count, 0)::bigint AS internal_reference_count,
       coalesce(reference_counts.external_count, 0)::bigint AS external_reference_count,
       internal.amount_minor, internal.currency, internal.minor_digits
FROM transactions AS internal
JOIN transactions AS external ON external.financial_account_id = internal.financial_account_id
    AND external.source_side = 'external'
    AND external.currency = internal.currency AND external.amount_minor = internal.amount_minor
    AND external.minor_digits = internal.minor_digits
    AND external.transaction_date BETWEEN internal.transaction_date - 3 AND internal.transaction_date + 3
LEFT JOIN reference_counts ON reference_counts.payment_reference = internal.payment_reference
    AND reference_counts.payment_reference = external.payment_reference
WHERE internal.financial_account_id = sqlc.arg(account_id)
  AND internal.source_side = 'internal'
  AND (sqlc.narg(internal_id)::uuid IS NULL OR internal.id = sqlc.narg(internal_id))
  AND (sqlc.narg(external_id)::uuid IS NULL OR external.id = sqlc.narg(external_id))
  AND (reference_counts.payment_reference IS NULL OR reference_counts.internal_count > 1 OR reference_counts.external_count > 1)
  AND NOT EXISTS (
      SELECT 1 FROM reconciliation_members
      WHERE transaction_id IN (internal.id, external.id) AND released_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM match_proposals
      WHERE internal_transaction_id = internal.id AND external_transaction_id = external.id AND status = 'rejected'
  );

-- name: CreateMatchProposal :exec
INSERT INTO match_proposals (financial_account_id, internal_transaction_id, external_transaction_id, evidence, job_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (internal_transaction_id, external_transaction_id) WHERE status = 'pending' DO NOTHING;

-- name: ListReviewQueue :many
SELECT sqlc.embed(transactions), sqlc.embed(financial_accounts), sources.name AS source_name, groups.pending_count
FROM (
    SELECT internal_transaction_id, count(*) AS pending_count
    FROM match_proposals
    WHERE status = 'pending'
    GROUP BY internal_transaction_id
) AS groups
JOIN transactions ON transactions.id = groups.internal_transaction_id
JOIN financial_accounts ON financial_accounts.id = transactions.financial_account_id
JOIN sources ON sources.id = transactions.source_id
ORDER BY transactions.transaction_date, transactions.id;

-- name: GetReviewTransaction :one
SELECT sqlc.embed(transactions), sqlc.embed(financial_accounts), sources.name AS source_name
FROM transactions
JOIN financial_accounts ON financial_accounts.id = transactions.financial_account_id
JOIN sources ON sources.id = transactions.source_id
WHERE transactions.id = $1 AND transactions.source_side = 'internal';

-- name: ListMatchProposals :many
SELECT sqlc.embed(match_proposals), sqlc.embed(transactions), sources.name AS source_name
FROM match_proposals
JOIN transactions ON transactions.id = match_proposals.external_transaction_id
JOIN sources ON sources.id = transactions.source_id
WHERE match_proposals.id = sqlc.narg(proposal_id)::uuid
   OR (match_proposals.internal_transaction_id = sqlc.narg(internal_id)::uuid AND match_proposals.status = 'pending')
ORDER BY (match_proposals.evidence->>'reference_relation' = 'shared') DESC,
         (match_proposals.evidence->>'days_apart')::integer,
         match_proposals.external_transaction_id;

-- name: GetMatchProposalRecord :one
SELECT * FROM match_proposals WHERE id = $1;

-- name: GetAcceptedMatchProposalID :one
SELECT id FROM match_proposals WHERE reconciliation_id = $1 AND status = 'accepted';

-- name: ResolveMatchProposal :one
UPDATE match_proposals
SET status = sqlc.arg(status),
    resolved_at = coalesce((SELECT r.confirmed_at FROM reconciliations AS r WHERE r.id = sqlc.narg(reconciliation_id)), statement_timestamp()),
    resolved_by = sqlc.arg(analyst_id),
    resolved_by_name = (SELECT u.display_name FROM users AS u WHERE u.id = sqlc.arg(analyst_id)),
    reconciliation_id = sqlc.narg(reconciliation_id)
WHERE match_proposals.id = sqlc.arg(id) AND match_proposals.status = 'pending'
RETURNING *;

-- name: InvalidateCompetingMatchProposals :exec
UPDATE match_proposals
SET status = 'invalidated',
    resolved_at = reconciliations.confirmed_at,
    resolved_by = reconciliations.confirmed_by,
    resolved_by_name = sqlc.narg(analyst_name),
    reconciliation_id = reconciliations.id
FROM reconciliations
WHERE reconciliations.id = sqlc.arg(reconciliation_id)
  AND match_proposals.financial_account_id = reconciliations.financial_account_id
  AND match_proposals.status = 'pending'
  AND (match_proposals.internal_transaction_id = sqlc.arg(internal_id)
       OR match_proposals.external_transaction_id = sqlc.arg(external_id));

-- name: CheckReprocessingAccount :one
SELECT id FROM financial_accounts
WHERE id = $1
FOR KEY SHARE;

-- name: AcquireAccountLease :execrows
INSERT INTO reconciliation_account_leases (financial_account_id, attempt_token, expires_at)
SELECT sqlc.arg(financial_account_id)::uuid, sqlc.arg(attempt_token)::uuid,
       clock_timestamp() + sqlc.arg(lease_duration)::interval
WHERE NOT EXISTS (
    SELECT 1 FROM reconciliation_account_leases AS active
    WHERE active.financial_account_id = sqlc.arg(financial_account_id)
      AND active.expires_at > clock_timestamp()
)
ON CONFLICT (financial_account_id) DO UPDATE
SET attempt_token = EXCLUDED.attempt_token,
    expires_at = clock_timestamp() + sqlc.arg(lease_duration)::interval
WHERE reconciliation_account_leases.expires_at <= clock_timestamp();

-- name: RenewAccountLease :one
WITH available AS (
    SELECT financial_account_id
    FROM reconciliation_account_leases
    WHERE financial_account_id = $1 AND attempt_token = $2 AND expires_at > clock_timestamp()
    FOR UPDATE SKIP LOCKED
), renewed AS (
    UPDATE reconciliation_account_leases AS lease
    SET expires_at = clock_timestamp() + sqlc.arg(lease_duration)::interval
    FROM available
    WHERE lease.financial_account_id = available.financial_account_id
    RETURNING 1
)
SELECT EXISTS (SELECT 1 FROM renewed) OR EXISTS (
    SELECT 1 FROM reconciliation_account_leases AS current_lease
    WHERE current_lease.financial_account_id = $1
      AND current_lease.attempt_token = $2 AND current_lease.expires_at > clock_timestamp()
) AS owned;

-- name: ReleaseAccountLease :exec
DELETE FROM reconciliation_account_leases
WHERE financial_account_id = $1 AND attempt_token = $2;

-- name: VerifyAccountLease :one
SELECT financial_account_id FROM reconciliation_account_leases
WHERE financial_account_id = $1 AND attempt_token = $2 AND expires_at > clock_timestamp()
FOR UPDATE;

-- name: GetJobSummary :one
SELECT * FROM reconciliation_job_summaries WHERE job_id = $1;

-- name: SaveJobSummary :exec
INSERT INTO reconciliation_job_summaries (
    job_id, financial_account_id, status, completed_at, attempt, max_attempts, automatic_count, proposal_count
) VALUES (
    sqlc.arg(job_id)::bigint, sqlc.arg(financial_account_id)::uuid,
    sqlc.arg(status), sqlc.arg(completed_at), sqlc.arg(attempt), sqlc.arg(max_attempts),
    (SELECT count(*) FROM reconciliations WHERE job_id = sqlc.arg(job_id) AND method = 'automatic'),
    (SELECT count(*) FROM match_proposals WHERE job_id = sqlc.arg(job_id))
)
ON CONFLICT (job_id) DO NOTHING;

-- name: ListOpenTransactions :many
WITH unique_references AS (
    SELECT financial_account_id, payment_reference
    FROM transactions
    WHERE payment_reference IS NOT NULL
    GROUP BY financial_account_id, payment_reference
    HAVING count(*) FILTER (WHERE source_side = 'internal') = 1
       AND count(*) FILTER (WHERE source_side = 'external') = 1
),
discrepant_pairs AS (
    SELECT internal.id AS internal_id, external.id AS external_id,
           -- Scale as numeric before subtraction to preserve precision beyond bigint.
           abs(internal.amount_minor * ('1e-' || internal.minor_digits)::numeric
             - external.amount_minor * ('1e-' || external.minor_digits)::numeric) AS amount
    FROM unique_references AS refs
    JOIN transactions AS internal USING (financial_account_id, payment_reference)
    JOIN transactions AS external USING (financial_account_id, payment_reference)
    WHERE internal.source_side = 'internal' AND external.source_side = 'external'
      AND internal.currency = external.currency
      AND NOT EXISTS (
          SELECT 1 FROM reconciliation_members
          WHERE transaction_id IN (internal.id, external.id) AND released_at IS NULL
      )
),
discrepancies AS (
    SELECT internal_id AS transaction_id, amount FROM discrepant_pairs WHERE amount <> 0
    UNION ALL
    SELECT external_id AS transaction_id, amount FROM discrepant_pairs WHERE amount <> 0
)
SELECT sqlc.embed(transactions), sqlc.embed(financial_accounts), sources.name AS source_name,
       coalesce(discrepancies.amount::text, '')::text AS discrepancy_amount,
       EXISTS (
           SELECT 1 FROM match_proposals
           WHERE status = 'pending'
             AND (internal_transaction_id = transactions.id OR external_transaction_id = transactions.id)
       ) AS has_pending_match_proposals
FROM transactions
JOIN financial_accounts ON financial_accounts.id = transactions.financial_account_id
JOIN sources ON sources.id = transactions.source_id
LEFT JOIN discrepancies ON discrepancies.transaction_id = transactions.id
WHERE NOT EXISTS (
    SELECT 1 FROM reconciliation_members
    WHERE transaction_id = transactions.id AND released_at IS NULL
)
ORDER BY transactions.transaction_date DESC, transactions.id;
