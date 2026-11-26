# Reconciliation demo data

These synthetic files describe activity on one **Financial Account**. For both imports, select Corporate Card (`corporate-card`). Select the preconfigured **Source** for each file and check the inferred **Import Coverage** in the preview.

| File | Side | Source | Import Coverage |
| --- | --- | --- | --- |
| [internal.csv](internal.csv) | Internal | Demo ERP | November 16–20, 2026 |
| [external.csv](external.csv) | External | Demo Card Statement | November 16–21, 2026 |

Both files use the six-column CSV format from the [Reconcilio MVP spec](https://github.com/kmmenna/reconcilio/issues/1). Negative amounts represent purchases on both sides. A blank `payment_reference` leaves the decision to analyst review when eligible candidates are found.

## Expected results

| Scenario | Records | Expected result |
| --- | --- | --- |
| Automatic confirmation | `ERP-001` and `CARD-001` | Both share the unique `DEMO-PAY-001` reference and a normalized amount of BRL -89.90. The engine confirms the pair and records its evidence. |
| Competing proposals | `ERP-002` with `CARD-002` or `CARD-003` | Both candidates have the same amount, currency, and date difference. Confirm `CARD-002` for the Canva purchase. The competing Adobe proposal is invalidated and remains in the history. |
| Amount discrepancy | `ERP-003` and `CARD-004` | Both transactions stay open. The open transactions list shows the BRL 1.00 difference linked by the shared `DEMO-PAY-003` reference. |

The same files work with the engine's evidence alone. With AI configured, the proposal review can also show a semantic ranking and explanations for the two candidates.
