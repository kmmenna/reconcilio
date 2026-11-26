# Persist match proposals for review

When a candidate is sent for analyst review, persist a match proposal with its own identity, the participating transactions, and the evidence presented. Record the analyst's decision when it is made so reprocessing cannot erase the basis for review. Intermediate candidates remain transient.

Store the proposal state and its single resolution on the proposal row. A proposal starts as pending and resolves as accepted, rejected, or invalidated. Preserve the original evidence, resolution time, analyst identity when applicable, and associated reconciliation and ranking references when present.

When a reconciliation is confirmed, mark its proposal as accepted if it was confirmed through analyst review. In the same atomic operation, invalidate every remaining pending proposal that shares a transaction with the reconciliation. Remove the invalidated proposals from the review queue and retain their evidence and a reference to the reconciliation that caused the invalidation.

Keep one pending proposal per transaction pair. When reprocessing finds the same pair, reuse the pending proposal and preserve its original evidence. Create proposals for newly discovered eligible pairs.

Manual confirmation and proposal publication acquire the same financial account row lock used by automatic confirmation. Rank candidates before acquiring the lock. Recheck pair eligibility under the lock and publish only still-eligible proposals in the same database transaction.

Reprocessing excludes any pair an analyst has rejected from proposal generation and automatic confirmation. Each transaction remains eligible to match other transactions. An explicit analyst action could reopen a rejected pair in a future release.

## Review queue

Group pending proposals by internal transaction. Each internal transaction with pending proposals appears once in the queue. In its review view, show all pending external alternatives and their evidence side by side. An external transaction can appear in more than one group when it is proposed for different internal transactions.

## Review API

| Endpoint | Behavior |
| --- | --- |
| `GET /api/review-queue` | Return one summary per internal transaction with pending proposals, including the pending proposal count. |
| `GET /api/transactions/{id}/match-proposals` | Return the internal transaction and all its pending proposals, including the external transactions, evidence for comparison, and the displayed ranking evaluation ID when available. |
| `GET /api/match-proposals/{id}` | Support direct links in all proposal states. Return the participating transactions, original evidence, current state, and associated reconciliation ID when the proposal was accepted or invalidated by a reconciliation. |
| `POST /api/match-proposals/{id}/confirm` | Confirm the proposal by ID, record the analyst's decision, and return the resulting reconciliation ID. |
| `POST /api/match-proposals/{id}/reject` | Reject the proposal by ID and record the analyst's decision. |

Both decision endpoints accept an optional `ranking_evaluation_id` identifying the evaluation shown to the analyst. Validate that the evaluation covers the proposal, then preserve the reference with the decision, even if a newer evaluation exists. Engine-only review omits the field. Apply the reconciliation eligibility rules independently of the ranking.

Repeating the recorded decision returns its original outcome, including the reconciliation ID for a confirmation. Preserve the original decision and its attribution. Return `409 Conflict` when the requested decision conflicts with the recorded decision or when a competing reconciliation has invalidated the proposal, so the analyst can refresh the available options.
