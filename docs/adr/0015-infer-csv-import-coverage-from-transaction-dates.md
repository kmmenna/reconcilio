# Infer CSV Import Coverage from Transaction dates

Derive Import Coverage during the backend CSV preview from the earliest and latest Transaction dates submitted for each Financial Account. The browser displays the applied interval in the preview and uses the existing import confirmation. This removes date entry from the standard upload and handles files whose rows are out of order.

## Consequences

The inferred interval includes both endpoints and covers the dates observed in the delivery. A Transaction redelivered on a different date without a stable Source Reference can fall outside the prior interval. An API caller can supply both endpoints when the Source provides an authoritative period, including days without activity.

Use the staged coverage for preview and confirmation checks against the same Source and Financial Account, following the overlap and replay rules in [ADR 0003](0003-separate-transaction-identity-from-import-deduplication.md). Retain the intervals already stored for previous imports and pending previews.
