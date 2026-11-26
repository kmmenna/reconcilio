# Reconcilio MVP

## Problem and approach

Financial Operations Analysts need to match internal ledger entries to external statements without hiding real discrepancies. Reconcilio turns each CSV into a reviewable preview and records the resulting decisions. The Corporate Card demo shows an automatic reconciliation, two competing proposals resolved by an analyst, and a same-reference amount difference that remains open.

## Decisions and trade-offs

The Go modular monolith uses PostgreSQL for exact signed Money in minor units, stable Transaction identities, and exclusive active Reconciliation membership. An import, its receipt, and its River job commit together. Workers use account-scoped leases and preserve durable job summaries. The model groups Reconciliation Members, with one member per side as the MVP policy. Next.js serves a bilingual browser workspace and handles session cookies at the BFF boundary. Local accounts, file imports, and a single database keep setup reproducible.

The engine auto-confirms only when a shared Payment Reference is unique on both sides of the account and Money agrees exactly. Other eligible pairs go to review. Reimports, duplicates, and retries preserve the original records and evidence.

## Use of AI

An optional ranker orders eligible alternatives and gives each candidate one score with explanations in English and Brazilian Portuguese. The application limits requests, minimizes the payload, validates results, and keeps engine evidence available when ranking is unavailable. Go applies all financial rules. During development, AI helped refine the domain, turn choices into acceptance criteria, and implement and review changes through separate pull requests.

## Verification and next steps

The demo runs locally without an AI key. Integration tests use PostgreSQL and a controlled ranker. Playwright covers imports, processing, review, and direct links in both languages. Future work includes event ingestion and live Sources, OIDC, and broader Reconciliation cardinalities.
