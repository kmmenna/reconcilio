# Additional test cases

## Reimport with an amount conflict

Start each check with both [demo files](../demo/README.md) imported and their automatic reconciliation completed. Upload [reimport-amount-conflict.csv](reimport-amount-conflict.csv) using `Demo ERP` as the **Source**, `Corporate Card` as the **Financial Account**, and November 16 through November 22, 2026, as the coverage period. Both internal files provide stable **Source References**, so the overlapping periods are eligible for import.

The import should show a conflict for `ERP-001`. The stored amount is BRL -89.90 and the incoming amount is BRL -94.90. The file also contains `ERP-004`, a new transaction for BRL -34.50.

| Analyst decision | Expected result |
| --- | --- |
| Keep the existing transaction and continue | `ERP-001` retains BRL -89.90 and its reconciliation with `CARD-001`. The import skips the conflicting row and creates `ERP-004`. |
| Cancel the import | `ERP-001` retains BRL -89.90 and its reconciliation with `CARD-001`. `ERP-004` remains absent. |

Verify that the import history records the analyst's decision in both cases.
