# Instrument the API and worker

Use Go's `log/slog` for structured application logs and OpenTelemetry for traces and metrics. Instrument HTTP requests, imports, reconciliation jobs, and AI calls. Correlate logs and traces using import and job identifiers, and track job durations, job outcomes, and AI fallbacks.

Use the `otelslog` bridge and `stdoutlog` exporter to emit JSON logs to stdout by default. Make stdout export for traces and metrics configurable. The local demo can run these exporters without an OpenTelemetry Collector.

## Request correlation

The frontend generates a request ID for each HTTP request and a correlation ID that stays the same across requests in a workflow. The Next.js BFF forwards both IDs to Go. The API preserves the IDs in logs and traces. When a request schedules a background job, persist the correlation ID and initiating request ID with the job so the worker can trace it back to the frontend workflow.

## Business and AI usage metrics

Derive business metrics from persisted records so retries and reprocessing do not count the same outcome twice. Track imported transactions, confirmed reconciliations by automatic or manual decision, rejected proposals, and open transactions by side.

Track AI call counts and durations, fallbacks by reason, and input and output tokens by provider and model. Use token counts reported by the provider when available.

For the MVP, use the OpenAI project's Spend dashboard to review monetary costs. Cost estimation per reconciliation job is a future capability.
