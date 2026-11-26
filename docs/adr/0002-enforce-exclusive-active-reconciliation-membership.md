# Enforce exclusive active reconciliation membership

A transaction may appear in multiple match proposals and belong to at most one active reconciliation. Confirmation must enforce exclusivity atomically across all participating transactions so concurrent confirmations that claim the same transaction for different reconciliations produce one successful confirmation and a conflict for the other. This preserves competing proposals for review and prevents a transaction from being reconciled twice.

Enforce exclusivity in PostgreSQL with a partial unique index on `reconciliation_members.transaction_id` where `released_at IS NULL`. Set `released_at` on all members in the same transaction when reversing a reconciliation so the original membership remains in history.

## Consequences

Analyst-initiated reversal is optional for the MVP and comes after the core import and reconciliation workflow in delivery priority.
