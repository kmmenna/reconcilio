# Use Go for the backend

Use Go for the API and worker so they can share the reconciliation module and run in the same process by default. Serve the API with `net/http` and `http.ServeMux`. Register HTTP operations with Huma v2 through its `humago` adapter so typed request and response definitions drive validation and generate the OpenAPI contract. HTTP handlers call the application's use cases, which own business rules.

## HTTP errors

Use RFC 9457 `application/problem+json` for HTTP errors with a stable machine-readable `code`. Return `422` for CSV row validation failures and include the line, field, and reason for each error. Return `409` when import confirmation detects new or changed conflicts and include the details the analyst needs to review.

## Application modules

The ingestion module owns imports, coverage, imported transactions, and import decisions. The CSV reader is an ingestion adapter that decodes files and preserves row positions. The reconciliation module reads imported transactions and owns eligibility rules, match proposals and their evidence, and confirmed reconciliations.

The identity module owns user accounts, credentials, login, access token issuance and validation, and profile changes. The catalog module owns financial accounts and sources and exposes the lookups that ingestion uses to resolve their identifiers.

## Go project layout

Within `apps/reconciliation`, use `cmd/reconcilio` as the entry point that starts the configured API and worker components. Place the business modules in `internal/ingestion`, `internal/reconciliation`, `internal/identity`, and `internal/catalog`. Keep SQL queries and their generated `sqlc` code with the owning module. Keep the application's database migrations in a common directory.

## Database access

Write PostgreSQL queries explicitly and use `sqlc` to generate typed Go methods for `pgx/v5`. Use `pgxpool` for database connections in the API and worker. Keep transaction boundaries explicit at the operations that persist an import and its job or confirm a reconciliation and resolve competing proposals.

## Schema migrations

Use Tern to apply versioned application SQL migrations in a dedicated Docker Compose step before starting the API and worker. In the same step, use `rivermigrate` from the application's River dependency to apply the queue migrations. Use the application migration files as the schema input for `sqlc` so generated queries and the running database share a single application schema definition.
