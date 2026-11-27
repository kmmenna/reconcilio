# Next steps

These proposals extend the MVP. The project tracks implementation scope and dependencies in GitHub Issues. [Issue tracker](https://github.com/kmmenna/reconcilio/issues) · [MVP scope](https://github.com/kmmenna/reconcilio/issues/1#out-of-scope)

## Export results for financial close

The analyst needs a result they can share, archive, or use to complete the close outside Reconcilio. The proposed first step is a CSV export for a selected Financial Account and period. It would include confirmed pairs with their available Source References, signed amounts and currencies, and whether the decision was automatic or manual. It would also include open Transactions and any known amount differences.

Validate the export against a real closing workflow. Confirm which dates define the export period, what downstream format is needed, and whether the analyst must be able to reproduce a past close. If analysts still need to update ERP records individually, evaluate writing the confirmed results back to the ERP.

## Bring in data from more Sources

- **Connect banks and ERPs.** Ingest records from live Sources and support near-real-time events where useful. Map external account identifiers to registered Financial Accounts, provision catalog entries where appropriate, and use stable Source References to avoid duplicate Transactions across CSV and connected deliveries. [Import identity and overlap rules](adr/0003-separate-transaction-identity-from-import-deduplication.md) · [Financial Accounts and Sources](adr/0005-reconcile-within-a-financial-account.md)
- **Accept more CSV formats.** Add browser-side preview and column mapping so analysts can work with Source files in their original format. Keep validation and confirmation in the backend. [CSV import decisions](adr/0003-separate-transaction-identity-from-import-deduplication.md)
- **Handle incomplete and corrected deliveries.** Allow partial acceptance of valid rows once incomplete Import Coverage can be tracked safely. Add a correction workflow for changed values on previously imported Transactions, preserving decision history and handling any active Reconciliation. [Reimport and correction decisions](adr/0003-separate-transaction-identity-from-import-deduplication.md)

## Expand reconciliation workflows

- **Match open Transactions manually.** Let analysts start from an open Transaction and search for a counterpart on the other side of the same Financial Account, including pairs missed by automated processing. Show matching clues such as signed amount, currency, dates, Payment Reference, and descriptions. Let the analyst confirm an eligible pair and record the evidence and who made the decision. Define the eligibility rules for analyst-initiated matches and preserve exclusive active membership. [Current matching policy](adr/0006-require-a-unique-shared-payment-reference-for-automatic-confirmation.md) · [Exclusive membership](adr/0002-enforce-exclusive-active-reconciliation-membership.md)
- **Match groups and handle currency conversion.** Evaluate one-to-many, many-to-one, and many-to-many Reconciliations. Treat cross-currency reconciliation as a policy decision that needs explicit FX rates, rounding, and evidence. [Reconciliation groups](adr/0001-model-reconciliation-as-a-group.md) · [MVP scope](https://github.com/kmmenna/reconcilio/issues/1#out-of-scope)
- **Revisit recorded decisions.** Allow analysts to reopen a rejected pair or reverse a confirmed Reconciliation. A reversal releases all members together and preserves the original decision and membership. [Match Proposals](adr/0004-persist-match-proposals-for-review.md) · [Exclusive membership](adr/0002-enforce-exclusive-active-reconciliation-membership.md)
- **Configure matching rules.** Identify which rules analysts need to vary and how changes should be governed. Preserve the evidence on earlier decisions as eligibility rules evolve. [Current matching policy](adr/0006-require-a-unique-shared-payment-reference-for-automatic-confirmation.md) · [MVP scope](https://github.com/kmmenna/reconcilio/issues/1#out-of-scope)

## Prepare for wider use

- **Add organizational sign-in.** Support OpenID Connect with a provider such as Keycloak and link external identities to existing user IDs so past decisions retain their attribution. [Identity decision](adr/0011-use-local-accounts-for-analyst-attribution.md)
- **Validate production operation.** Test matching and list queries with representative account volumes, paginate the review queue and results where needed, and add readiness checks for the API and worker. Evaluate hosted deployment and external observability. Per-job AI cost estimates are also a possible telemetry extension. [Telemetry decision](adr/0012-instrument-the-api-and-worker.md)
- **Revisit infrastructure as demand grows.** Evaluate separate services or databases and an external broker if deployment or processing requirements justify them. Preserve reliable job scheduling across any new boundary. Revisit dedicated monorepo tooling if CI or cross-project dependencies warrant it. [Job scheduling decision](adr/0008-queue-reconciliation-atomically-with-confirmed-imports.md) · [Build tooling decision](adr/0014-share-development-tasks-between-local-work-and-ci.md)
