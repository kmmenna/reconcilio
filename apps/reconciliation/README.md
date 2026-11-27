# Reconciliation backend

The Go backend imports and validates Transactions, reconciles eligible pairs, records analyst decisions, and processes account-level jobs in the background.

## Architecture

Reconcilio is a modular monolith. The `reconcilio` command assembles the HTTP API and River worker around the same business modules and PostgreSQL database. Both run in one process by default and can also run independently against the shared database. This keeps transaction boundaries explicit and gives the API and worker room to be deployed separately. [Application entry point](cmd/reconcilio/main.go) · [Backend ADR](../../docs/adr/0009-use-go-for-the-backend.md)

The code is organized by business capability. HTTP handlers translate requests and responses, use cases coordinate business decisions and transactional writes, and each module keeps its SQL and generated queries nearby. Invariants such as exclusive active Reconciliation membership are enforced in PostgreSQL. Focused interfaces let ingestion schedule jobs and the worker use a configurable ranking provider. New Sources and ranking providers can reuse the existing import and reconciliation rules through those module interfaces. [Database invariants](migrations/003_confirmed_imports_and_reconciliations.sql) · [Job scheduling interface](internal/ingestion/confirm.go) · [Ranking interface](internal/reconciliation/ranking.go)

## Stack choices

| Tool | Why it was chosen |
| --- | --- |
| **Go, `net/http`, and Huma** | One Go binary can serve the API and run the worker. The standard HTTP router handles routing, and Huma uses typed requests and responses to validate inputs and generate OpenAPI. Errors use `application/problem+json` with stable codes. |
| **PostgreSQL, pgx, and sqlc** | PostgreSQL stores signed Money in integer minor units and enforces uniqueness and transactional consistency. pgx exposes the pool and transaction boundaries, and sqlc turns explicit SQL into typed Go methods. |
| **Tern and `rivermigrate`** | Versioned application and queue migrations bring the database to the same schema in local development, Compose, and integration tests. |
| **River** | The durable queue lives in PostgreSQL, so an import can save its data and enqueue processing atomically. River manages job execution, retry scheduling, and recovery after a worker interruption. |
| **Argon2id and signed JWTs** | Local accounts support password hashing and stable analyst attribution. The API validates bearer tokens, and the frontend BFF manages the browser session cookie. |
| **`slog` and OpenTelemetry** | Structured logs, traces, and metrics correlate HTTP requests, imports, jobs, and AI calls. Optional stdout exporters make traces and metrics available in local runs. |

## Key components

| Module | Responsibility |
| --- | --- |
| [`internal/ingestion`](internal/ingestion/) | Parse CSVs, infer Import Coverage per Financial Account, identify replays and conflicts, and confirm or cancel imports. |
| [`internal/reconciliation`](internal/reconciliation/) | Evaluate candidates, confirm exact matches, publish proposals, track open Transactions, and preserve Match Evidence and decision history. The module also contains the River worker and job status handling. |
| [`internal/catalog`](internal/catalog/) | Manage Financial Accounts and Sources, resolve import assignments, and coordinate locks for changes that span modules. |
| [`internal/identity`](internal/identity/) | Manage local users, password changes, token issuance and validation, and administration. |
| [`internal/telemetry`](internal/telemetry/) | Record correlated operations, job outcomes, ranking outcomes, and business counts from persisted records. |
| [`internal/openairank`](internal/openairank/) | Implement `CandidateRanker` using the OpenAI Responses API. Ranking is optional and the engine supplies evidence and ordering when the provider is unavailable. [AI integration](docs/AI_INTEGRATION.md) |

### Follow two requests through the code

- **Confirm an import.** `POST /api/imports/{id}/confirm` calls the ingestion use case, which locks the affected accounts, rechecks conflicts, and writes the Transactions, Import Coverage, receipt, and River jobs in the same transaction. If revalidation changes the conflicts, it saves the updated preview before returning `409`. [HTTP handler](internal/ingestion/http.go) · [Use case](internal/ingestion/confirm.go) · [SQL](internal/ingestion/queries.sql) · [Integration test](cmd/reconcilio/import_preview_test.go)
- **Decide a proposal.** `POST /api/match-proposals/{id}/confirm` or `/reject` resolves a recorded proposal. Confirmation rechecks eligibility under the account lock, saves the Reconciliation, and invalidates competing proposals that share either Transaction. The original evidence and any linked ranking remain available in the history. [HTTP handler](internal/reconciliation/proposals_http.go) · [Use case](internal/reconciliation/proposal_decisions.go) · [SQL](internal/reconciliation/queries.sql) · [Integration scenarios](cmd/reconcilio/proposal_review_test.go)

## Jobs and the PostgreSQL queue

1. **Enqueue.** The import use case uses River’s `InsertTx` to record one job per affected Financial Account alongside the confirmed import. The receipt returns the job IDs. Analysts can also enqueue reprocessing for the open Transactions of one account. [Queue implementation](internal/reconciliation/queue.go) · [Reprocessing endpoint](internal/reconciliation/job_http.go)
2. **Coordinate by account.** A worker acquires an expiring PostgreSQL lease with a token unique to its attempt and renews it while working. A job for a busy account is deferred with its original job ID. Other accounts can progress concurrently. The worker checks lease ownership and expiry inside the transaction that writes results, so a superseded attempt cannot commit them. [Account lease](internal/reconciliation/account_attempt.go)
3. **Retry and keep committed results.** River allows up to three attempts, including the initial one. Repeated work preserves existing decisions and proposals. If the final attempt fails after committing some results, the job is marked failed and reports those results. Reprocessing evaluates the Transactions still open. [Queue and recovery ADR](../../docs/adr/0008-queue-reconciliation-atomically-with-confirmed-imports.md)
4. **Report and recover.** `GET /api/jobs/{id}` reads live state from River and final state from an application-owned summary. A recovery task runs before worker startup and periodically to fill in missing summaries. It derives counts from results linked to the original job ID. Summaries stay available after River removes its terminal job records, which it retains for 30 days. [Job status and recovery](internal/reconciliation/job_status.go)

## Observability

The API and worker use `slog` through the OpenTelemetry log bridge to emit structured JSON to stdout. The HTTP middleware instruments requests, and the application adds spans and metrics for import operations, job attempts, and AI ranking. The log handler records error types or PostgreSQL error codes in place of raw errors that may contain input or connection details. [Telemetry runtime](internal/telemetry/runtime.go) · [HTTP instrumentation](internal/telemetry/http.go) · [Telemetry ADR](../../docs/adr/0012-instrument-the-api-and-worker.md)

The Next.js BFF forwards `X-Request-ID` and `X-Correlation-ID`. The request ID identifies one HTTP request, and the correlation ID follows the workflow across requests. When an import enqueues a River job, its metadata preserves the initiating request ID, correlation ID, import ID, and trace context. The worker restores that context and adds the job ID, account ID, and attempt so the import and its background work can be traced together. [Correlation and job metadata](internal/telemetry/context.go)

| Area | What you can monitor |
| --- | --- |
| **HTTP API** | Method, route, response status, and duration in `HTTP request` logs. `http.server.request.duration` measures request latency. |
| **Imports** | Operation counts and duration by operation and outcome, including confirmation, replay, rejection, and failure. `Import operation` logs include the import ID, Transaction count, and queued job IDs when available. Metrics use `reconcilio.import.*`. |
| **Jobs** | Attempt counts and duration by outcome, including deferral, retry, completion, cancellation, and failure. `Reconciliation job` logs identify the account, job, and attempt. Metrics use `reconcilio.job.*`. |
| **AI ranking** | Outbound requests and duration by provider, model, and outcome. Fallbacks are counted by reason, and token metrics use provider-reported input and output counts when available. Metrics use `reconcilio.ai.*`. |
| **Business results** | Imported Transactions, confirmed Reconciliations by automatic or manual method, rejected proposals, and open Transactions by internal or external side. These `reconcilio.business.*` gauges come from a PostgreSQL snapshot, so retries and replay leave the totals consistent with persisted results. |

Set `RECONCILIO_TRACE_STDOUT=true` and `RECONCILIO_METRIC_STDOUT=true` independently to export traces and metrics to stdout. Metrics are collected every 10 seconds. In a split deployment, exporters connected to the same database each report the same database-wide business totals. Use one series per database for those gauges. [Local telemetry walkthrough](../../README.md#console-telemetry) · [Business metrics](internal/telemetry/business.go)

## Test strategy

`mise run check:go` checks formatting and runs `go vet`. `mise run test` uses Go’s `testing` package and starts real PostgreSQL instances with Testcontainers for integration coverage. Tests apply the same application and River migrations as local runs. Most workflows are exercised through HTTP handlers with `httptest`, using real database constraints, transactions, and queue behavior. Provider calls use a controlled ranker or a local HTTP test server. [Test strategy ADR](../../docs/adr/0013-test-with-postgresql-and-a-controlled-ai-provider.md) · [Test environment](../../docs/environment.md#tests)

| Area | Coverage |
| --- | --- |
| **Identity** | Administrator bootstrap, sign-in, token validation, profile updates, password changes, and access restrictions. [Identity tests](internal/identity/http_test.go) |
| **CSV imports and catalog** | Row errors, exact Money, inferred coverage, multi-account imports, replay and overlap rules, reimport conflicts, and rollback when queue insertion fails. [Import tests](cmd/reconcilio/import_preview_test.go) · [Catalog tests](cmd/reconcilio/catalog_import_test.go) |
| **Matching and review** | Exact matches, concurrent proposal decisions, exclusive membership, open Transactions, amount discrepancies, and preserved evidence. [Proposal scenarios](cmd/reconcilio/proposal_review_test.go) · [Open Transaction scenarios](cmd/reconcilio/open_transactions_test.go) |
| **Worker recovery** | Reprocessing, parallel accounts, deferral for a busy account, lost lease ownership, retry with partial results, and summary recovery. [Job tests](cmd/reconcilio/job_recovery_test.go) |
| **Ranking and telemetry** | Redaction, bilingual assessments, quotas, invalid responses, timeout and fallback, the provider HTTP contract, and trace correlation from import through worker execution. [Ranking tests](cmd/reconcilio/ranking_test.go) · [Adapter tests](internal/openairank/client_test.go) · [Telemetry tests](cmd/reconcilio/telemetry_test.go) |

Run `mise run reconciliation:dev` from the repository root to start the API and worker locally. The task starts the worktree’s PostgreSQL and applies both schemas. [Host development](../../README.md#run-the-go-api-on-the-host) · [Environment settings](../../docs/environment.md)
