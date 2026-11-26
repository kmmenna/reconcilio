# Require a unique shared payment reference for automatic confirmation

Automatically confirm a pair only when both transactions share the same payment reference, that reference appears once on each side of the same financial account, and both transactions have exactly equal normalized amounts in the same currency. Apply the existing restrictions on active membership and rejected pairs to automatic confirmation. Other eligible pairs may be proposed for analyst review based on available evidence.

Use the same financial account row lock as import confirmation. After acquiring the lock, recheck payment reference uniqueness and pair eligibility against current data, then persist the automatic confirmation in the same database transaction. Hold the lock only during the final recheck and write so work on different financial accounts can proceed independently.

Evaluate uniqueness at confirmation time. If a later import adds another transaction with the same payment reference, preserve the existing reconciliation and its recorded evidence. The new transaction enters the open workflow, and future automatic decisions use the updated reference counts.

## Candidates for review

For pairs without a shared payment reference or with a shared reference that occurs more than once on either side, generate candidates across the internal and external sides of the same financial account when the normalized amount and currency are equal and the dates are at most three calendar days apart. The date window applies in both directions so a search can start from either transaction.

For engine ordering within a review group, place candidates with a shared payment reference first, then sort by increasing absolute date difference. Break remaining ties by external transaction ID.

## Amount discrepancies

When two open transactions share a payment reference that occurs exactly once on each side of the same financial account, show the absolute amount difference if their currencies match and their normalized amounts differ. Calculate the difference when querying open transactions. Both transactions remain open. The pair is ineligible for a match proposal or reconciliation.

## Open transactions API

`GET /api/transactions?status=open` returns internal and external transactions with no active reconciliation. Include transactions with pending match proposals and indicate whether each transaction has any. Include the amount discrepancy when the shared payment reference qualifies under the rule above.
