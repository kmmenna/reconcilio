# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Finance operations analysts handling recurring reconciliation are the primary users. They import records from internal systems and external statements, check which transactions correspond, and resolve ambiguous matches and import conflicts. Administrators manage local user accounts.

## Product Purpose

Reconcilio establishes which internal and external financial records correspond within a financial account. A successful reconciliation cycle leaves analysts with confirmed pairs and the evidence behind each decision, pending proposals they can act on, and open transactions with any known amount discrepancies visible.

## Positioning

Reconcilio combines strict rules for automatic confirmation with an evidence-led review queue. A unique shared payment reference can establish a match when the account, currency, and signed amount agree. For ambiguous matches, analysts can compare eligible alternatives side by side. Optional AI ranking adds explanations, and engine evidence supports the same decisions when AI is unavailable.

## Operating Context

The MVP workflow starts with registered financial accounts and internal or external sources. The analyst selects a source and declares the CSV's coverage period. A single-account file uses a selected account, while a multi-account file uses registered account codes on its rows. The analyst previews the resolved accounts and transactions, addresses conflicts, and confirms the import.

Each affected account gets a background reconciliation job that the analyst can track. The analyst can also reprocess an account's open transactions. The review queue groups pending match proposals by internal transaction and shows the competing external records with their evidence. Analysts can confirm or reject proposals, inspect open transactions, and revisit proposal and reconciliation details through direct links.

## Capabilities and Constraints

- The MVP pairs one internal and one external transaction within the same financial account, with exactly equal signed amounts in the same currency. Each transaction can belong to at most one active reconciliation. [Reconciliation policy](docs/adr/0001-model-reconciliation-as-a-group.md) · [Exclusive membership](docs/adr/0002-enforce-exclusive-active-reconciliation-membership.md)
- A source reference identifies a record across deliveries within its source and account. A payment reference can be shared across sides and provides match evidence. Automatic confirmation requires the payment reference to occur exactly once on each side of the account. When an otherwise qualifying pair differs in amount, both transactions stay open and the amount difference is visible. [Automatic confirmation and discrepancies](docs/adr/0006-require-a-unique-shared-payment-reference-for-automatic-confirmation.md)
- Row validation errors block confirmation of the CSV. When a reimport conflicts with a stored transaction, the analyst can keep the stored record and continue without the conflicting row, or cancel the import. [CSV validation and reimports](docs/adr/0003-separate-transaction-identity-from-import-deduplication.md)
- Match proposals retain their evidence and decision history. Confirming a match invalidates competing pending proposals that share either transaction. Reprocessing excludes rejected pairs while leaving each transaction eligible to match another. Manual decisions are attributed to the signed-in analyst and automatic decisions to the reconciliation engine. [Match proposals](docs/adr/0004-persist-match-proposals-for-review.md) · [Decision attribution](docs/adr/0011-use-local-accounts-for-analyst-attribution.md)
- AI ranks and explains eligible alternatives. Eligibility and confirmation rules come from the engine, and every eligible alternative remains accessible to the analyst. Ranking requests use temporary candidate IDs and descriptions with detected personal, account, and card details replaced by placeholders. [AI-assisted review](docs/adr/0007-use-ai-to-assist-analyst-review.md)

## Evidence on Hand

- The glossary defines the domain terms used in the interface and product documentation. The ADRs describe intended MVP behavior. [Glossary](GLOSSARY.md) · [ADRs](docs/adr/)
- The synthetic demo CSVs pair an ERP export with a card statement for a Corporate Card account. They exercise automatic confirmation, competing proposals, and an amount discrepancy. A separate fixture exercises an amount conflict against a stored transaction. Both document expected outcomes for the planned application. [Demo CSVs](examples/demo/README.md) · [Reimport fixture](examples/test-cases/README.md)

## Product Principles

- Automate confirmation when the records meet the explicit matching rules.
- Give analysts the evidence and alternatives they need to decide ambiguous cases.
- Preserve the record of what was decided, who decided it, and what evidence they had at the time.
- Keep unresolved work visible and actionable across imports, background jobs, and analyst review.
