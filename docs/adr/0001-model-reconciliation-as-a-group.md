# Model reconciliation as a group

The MVP pairs one internal financial record with one external financial record. We model a reconciliation as a group of participating records so the one-to-one limit can be enforced as a business policy. A direct link would be simpler for the first release, but supporting other cardinalities later would require changing how existing reconciliation decisions are represented.

A reconciliation owns its members. Each member references a transaction and records its side. Confirmation records both members atomically in the MVP. Any reversal applies to the entire reconciliation and preserves its original membership in history.

An MVP reconciliation requires both transactions to have the same currency and exactly equal signed amounts under the shared sign convention. This rule applies to automatic confirmations and analyst confirmations.

Represent money as a currency code and a signed count of that currency's minor units. Use `int64` in Go and `bigint` in PostgreSQL for the count. Parse decimal CSV amounts directly into minor units and report amounts that exceed the currency's supported decimal precision as row validation errors.

Expose money in API JSON as a decimal string and a currency code, for example `{"amount":"-89.90","currency":"BRL"}`. This preserves the exact value through clients with different JSON number precision.

## Decision evidence

Persist match evidence as a `JSONB` document on the record that owns it. Define and validate its structure in Go. Keep identifiers, lifecycle states, and references in relational columns.

Store the evidence for an automatic confirmation on the reconciliation. For a manual confirmation, link the reconciliation to the accepted match proposal and read its preserved evidence through that link.

When the analyst reviewed an AI ranking, include the evaluation linked to the recorded proposal decision in the evidence for the manual confirmation.

## Reconciliation API

`GET /api/reconciliations` lists confirmed reconciliations and supports filtering by financial account. Each item includes the reconciliation ID, amount and currency, confirmation time, and whether the decision was automatic or manual. The UI links each result to its detail page.

`GET /api/reconciliations/{id}` supports direct links and returns the participating transactions, the decision time, the decision actor, and the evidence recorded at confirmation. Include the match proposal ID when an analyst confirmed a proposal. Reprocessing preserves the evidence presented for the original decision.
