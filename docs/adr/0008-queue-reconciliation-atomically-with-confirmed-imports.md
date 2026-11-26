# Queue reconciliation atomically with confirmed imports

When an import is confirmed, persist the imported data and one durable reconciliation job per affected financial account in the same atomic operation. Return the job IDs with the import result so the analyst can track background processing separately for each account. Analysts can also request manual reprocessing of open transactions through the same background workflow.

Use the open-source River Go library with its `pgx/v5` driver and the application PostgreSQL database. Use River's `InsertTx` to enqueue jobs in the import confirmation transaction. River manages job state, retries, and recovery after worker interruption. The worker invokes the reconciliation module through its application interface. The module owns matching and confirmation rules and applies changes idempotently across job attempts.

Allow at most three attempts per job, including the initial attempt. Use River's default retry delays for transient processing failures. If AI ranking fails, continue processing with the engine's ordering and evidence.

Enforce at most one job in reconciliation processing per financial account across workers at the application level. If a worker claims another job for a busy account, defer that job through River and keep its ID. Jobs for different financial accounts can run concurrently.

Use an expiring PostgreSQL lease per financial account. Acquire and renew the lease in short transactions and identify its owner with a token unique to the job attempt. Release the lease only if the attempt still owns it. Return the connection to the pool between database operations, including while waiting for AI. Before persisting reconciliation effects, lock the lease row and verify that the attempt owns an unexpired lease in the same transaction as the write. An attempt that loses its lease must discard any uncommitted work.

Build one Go binary for the MVP with the API and worker enabled by default in the same process. Keep their startup routines separate and allow each component to be enabled or disabled independently at startup.

## Manual reprocessing API

`POST /api/financial-accounts/{id}/reprocess` enqueues a reconciliation job for the account's open transactions. Return `202 Accepted` with the job ID after the request is durably queued. The analyst follows the job through `GET /api/jobs/{id}`.

## Job history

Persist a final job summary in application-owned PostgreSQL data, keyed by the job ID. Record the financial account, final status, completion time, and counts of automatic reconciliations and match proposals created by the job. Count each committed result once across all attempts.

Store the creating job ID when inserting an automatic reconciliation or a match proposal. Keep the original job ID if a later attempt or job reuses the record. Derive the summary counts from those persisted results, and retain their job IDs independently of River's job retention.

If the job exhausts its attempts after committing some results, keep those results and show the job as failed with partial results. The summary includes the committed counts. A later reprocessing job evaluates the transactions that are still open.

Run a recovery task before starting the River worker and periodically during execution. Find finalized River jobs with a missing summary, read the final status and completion time from River, derive the counts from committed results, and persist the summary idempotently.

`GET /api/jobs/{id}` returns the current execution status from River while the job is in progress and the persisted summary for a finalized job. The summary stays available after River removes the job.

Configure River to retain completed, cancelled, and discarded jobs for 30 days. This bounds retention of technical attempt history and errors. If summary recovery stays unavailable throughout that window while River cleanup continues, a missing summary can lose its source job record.

## Consequences

The same binary can run as separate API-only and worker-only processes against the shared database. Moving modules to services with independent databases would require a durable handoff that preserves the guarantee of scheduling reconciliation for every confirmed import.
