# Reconcile within a financial account

Resolve the financial account for every transaction at ingestion, and generate candidates and confirm reconciliations only within that account.

## Account and source setup

Allow any authenticated user to view and create financial accounts and sources in the MVP interface. A financial account has a name and a stable code. A source has a name and a side, internal or external. The local demo provides the Corporate Card account and its two sources, Demo ERP and Demo Card Statement.

Authenticated users can edit financial account and source names at any time. The financial account code is immutable after creation. A source's side can be edited until its first confirmed import.

A source can be used with multiple registered financial accounts. An import selects one source and can include transactions for multiple accounts. Show the selected source and resolved accounts in the preview for the analyst to review before confirmation.

## Catalog API

Use `GET /api/financial-accounts` and `GET /api/sources` for the catalog and import selectors. Use `POST` on the same collection paths to create records. Use `PATCH /api/financial-accounts/{id}` to edit the account name and `PATCH /api/sources/{id}` to edit the source name or its side when the source is still eligible for a side change.

## CSV format

Use six base columns in the MVP CSV format: `source_reference`, `payment_reference`, `transaction_date`, `amount`, `currency`, and `description`. Use `YYYY-MM-DD` dates, decimal amounts with a period, and three-letter currency codes. Use negative amounts for outflows and positive amounts for inflows. A source reference or payment reference can be blank when unavailable.

For a single-account CSV, the analyst selects the financial account during import. For a multi-account CSV, add the `financial_account` column and require a registered financial account code in each row. Use stable, human-readable codes such as `corporate-card`. For an unknown code, report the affected CSV row and the code that could not be resolved.

## Consequences

A credit card account is scoped to the statement and can cover multiple individual cards. Masked card details can provide additional matching evidence. Future integrations can populate financial accounts and sources from external systems and resolve provider account identifiers to the same financial accounts used by CSV imports.
