# Separate transaction identity from import deduplication

Every transaction has a permanent internal identity used by reconciliations. When a stable reference is available, use the source, financial account, and source reference together to identify repeated deliveries of the same record across exports and integration channels. This lets a source reuse references across accounts.

Enforce that identity in PostgreSQL with a partial unique index on `(source_id, financial_account_id, source_reference)` for rows with a non-null source reference. Represent an absent source reference as null and apply the import coverage rules to those records.

The MVP accepts CSVs with and without stable references from the same source. Record coverage by source and financial account, including each account represented in a mixed CSV. Validate that the records fall within the declared period. Recognize an identical file replay with the same source and account assignments before checking for overlaps. For each source and account, allow overlapping coverage only when both imports provide stable references for every record in that account. Matching financial fields alone can collapse distinct transactions.

Row validation errors such as invalid fields or unknown financial account codes block confirmation of the entire CSV. Report the affected rows so the analyst can correct and resubmit the file. Record transactions and coverage only after validation passes.

When rows in the same CSV share a source reference within a financial account and no stored transaction exists for that identity, collapse them into one transaction if all normalized fields agree. If any normalized field differs, return `422`, identify the conflicting rows, and block the import so the analyst can correct the source file.

If an incoming record has the same source, financial account, and source reference as an existing transaction but differs in amount, currency, transaction date, or payment reference, report the conflicting values to the analyst. The analyst can keep the existing transaction and continue the import without the conflicting row, or cancel the import. Record the decision in the import history.

If only the description differs, treat the incoming row as a replay, retain the original description, and let the import proceed without an analyst decision for that row.

## CSV processing

The Go API receives the CSV and calculates its replay fingerprint from the received bytes. A CSV reader decodes records and keeps their row positions for validation errors. The import use case handles validation and persistence and can receive records from other ingestion adapters. The React frontend displays the preview and row errors returned by the API.

Persist a pending import in PostgreSQL with the file fingerprint, decoded records, source side shown in the preview, and conflicts found during preview. Use its import ID to retrieve the preview, confirm it, or cancel it. On confirmation, recheck the stored source side, coverage, and source-reference conflicts against the current database state before committing the transactions, coverage, and reconciliation request.

Store the preview rows and conflicts in one `JSONB` field per pending import. Persist accepted transactions and coverage in relational tables. Clear the temporary preview when the import is confirmed or canceled and keep import metadata and recorded decisions for audit.

During confirmation, lock the affected financial account rows before rechecking coverage and inserting transactions. Acquire locks in a consistent account ID order for multi-account imports and hold them until the confirmation transaction ends. This serializes confirmations that share an account while allowing confirmations for separate accounts to proceed independently.

## HTTP import workflow

| Operation | Contract |
| --- | --- |
| `POST /api/imports` | Accept a multipart CSV upload with the source, coverage period, and selected account for single-account files. Return the preview and import ID or row validation errors. |
| `GET /api/imports/{id}` | Return the pending preview or the recorded outcome once resolved. |
| `POST /api/imports/{id}/refresh-preview` | Regenerate a pending preview from its stored records and return the new preview and its own import ID. |
| `POST /api/imports/{id}/confirm` | Accept the IDs of the conflicts the analyst resolved by keeping the original transaction, recheck the import, and return the recorded result and job IDs. Repeating confirmation for a confirmed import returns the same result and job IDs. |
| `POST /api/imports/{id}/cancel` | Cancel a pending import and record the decision in its history. |
| `GET /api/jobs/{id}` | Return processing status or the persisted final summary for the affected financial account. |

Give each preview conflict an ID that identifies the discrepancy presented to the analyst. Confirm using the stored preview and the submitted conflict IDs. If revalidation finds a new or changed conflict, return `409 Conflict` with the new details and keep the import pending for review.

Use the source side stored in the preview for comparison on confirmation. If the current source side differs, return `409 Conflict` with the previewed and current sides and keep the original preview pending.

When the analyst refreshes the preview after a source side change, reuse the decoded records, revalidate against current data, and create a new preview with the current source side and a new import ID. The original preview ID continues to identify the original side seen by the analyst.

## Consequences

A future event integration can reuse the same source references. For each source and account with reference-free history, start after the last covered period or explicitly link overlapping deliveries to the historical transactions before processing them.

Accepting an incoming value as a correction is a future capability. A correction workflow needs to preserve decision history and address any active reconciliation involving the corrected transaction.

Accepting valid rows from a CSV with validation errors is a future capability. It requires tracking incomplete coverage so corrected rows can be imported safely.

Browser-side CSV reading and column mapping are future capabilities. This would let analysts preview and map source columns in the browser before submission. The backend remains responsible for validating the resulting records and committing the import.
