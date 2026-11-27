# AI integration

The reconciliation engine can ask a model to rank the eligible alternatives for one internal Transaction. The engine determines eligibility and automatic matches, and the analyst decides which proposal to accept or reject. Every eligible alternative remains available with engine evidence. [AI-assisted review ADR](../../../docs/adr/0007-use-ai-to-assist-analyst-review.md)

## Model choice

The local MVP configuration uses GPT-5 nano (`gpt-5-nano`). It fits the task because the model compares short descriptions in a bounded group and explains the ranking in two languages. OpenAI describes it as suited to classification and summarization, and it supports Structured Outputs on the Responses API, which lets the adapter require the bilingual JSON structure. The model is selected through `OPENAI_MODEL`. OpenAI marks the dated `gpt-5-nano-2025-08-07` snapshot as deprecated. [Model documentation](https://developers.openai.com/api/docs/models/gpt-5-nano) · [Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs?api-mode=responses)

## Request and response

1. **Prepare a group.** The engine selects candidates that satisfy the account, amount, currency, and date rules. When the current group needs a new assessment, it prepares one request for all eligible alternatives. The payload contains the internal description and the external descriptions with temporary candidate IDs. These temporary IDs stand in for the original Transaction IDs, and the backend keeps the mapping local to the request. This limits the exposure of identifying data to the provider. Detected personal information and account or card details within descriptions are replaced with request-scoped placeholders. [Ranking preparation](../internal/reconciliation/ranking.go) · [Redaction](../internal/reconciliation/ranking_redaction.go)
2. **Call OpenAI.** The `CandidateRanker` adapter sends a bearer-authenticated `POST` to the Responses API with a strict JSON Schema. The adapter requests `store: false` and treats descriptions as input data. It expects one ordered `ranking` array containing each candidate ID, an integer score from `0` to `100`, and short explanations in `pt-BR` and `en`. The position in the array determines the rank. [OpenAI adapter](../internal/openairank/client.go)
3. **Validate and publish.** The backend checks that the response includes each supplied ID exactly once, validates scores and both explanations, and sanitizes echoed sensitive detail. The worker calls the provider outside the transaction that writes results. It rechecks eligibility before publishing and saves the assessment only if it still covers the review group. [Account processing](../internal/reconciliation/process_account.go) · [Application validation](../internal/reconciliation/ranking.go) · [Proposal publication](../internal/reconciliation/proposals.go)
4. **Retain the decision context.** PostgreSQL stores successful assessments separately from proposal evidence, with both explanations under the same rank and score. A covering assessment can be reused during reprocessing. If a new alternative joins the group, the next evaluation ranks the complete group. Language changes use the saved explanations, and a decision can retain the ID of the assessment shown to the analyst. [Ranking storage](../migrations/008_match_ranking.sql) · [Decision recording](../internal/reconciliation/proposal_decisions.go)

### Example

This example uses descriptions from the synthetic Corporate Card demo. The Canva statement description includes a placeholder for a synthetic card detail. The adapter serializes this JSON into the Responses API’s `input` string and sends it with the model, instructions, and strict JSON Schema.

```json
{
  "internal_description": "Canva subscription for the design team",
  "candidates": [
    {
      "candidate_id": "c_JBSWY3DPEHPK3PXP7ZQ3A5B6CD_1",
      "description": "ADOBE* CLOUD"
    },
    {
      "candidate_id": "c_JBSWY3DPEHPK3PXP7ZQ3A5B6CD_2",
      "description": "CANVA* I045 [redacted_JBSWY3DPEHPK3PXP7ZQ3A5B6CD_1]"
    }
  ]
}
```

The adapter reads the JSON from the assistant’s `output_text` item in `output[].content[]`. In this illustrative response, the second candidate in the input is ranked first.

```json
{
  "ranking": [
    {
      "candidate_id": "c_JBSWY3DPEHPK3PXP7ZQ3A5B6CD_2",
      "score": 93,
      "explanations": {
        "pt-BR": "As duas descrições mencionam o Canva, o que favorece a correspondência.",
        "en": "Both descriptions mention Canva, which supports the match."
      }
    },
    {
      "candidate_id": "c_JBSWY3DPEHPK3PXP7ZQ3A5B6CD_1",
      "score": 24,
      "explanations": {
        "pt-BR": "O extrato menciona a Adobe, um fornecedor diferente.",
        "en": "The statement names Adobe, a different merchant."
      }
    }
  ]
}
```

## Controls and fallback

| Control | Behavior |
| --- | --- |
| **Configuration** | Both `OPENAI_API_KEY` and `OPENAI_MODEL` enable ranking. `OPENAI_RESPONSES_URL` can point to a controlled endpoint for testing. |
| **Group and response bounds** | A request covers at most 12 candidates and 12 KiB of prepared input. The adapter requests up to 6,000 output tokens and caps the HTTP response body at 256 KiB. |
| **Attempt and time limits** | One outbound attempt per eligible evaluation, with a 15-second timeout by default. The worker may shorten the timeout to leave time for publication. |
| **Usage limits** | PostgreSQL atomically reserves each attempt before the network call. The defaults are five attempts per job and 50 per UTC day, including failed calls. `RECONCILIO_AI_PER_JOB_LIMIT` and `RECONCILIO_AI_DAILY_LIMIT` override those defaults. |
| **Fallback and telemetry** | Disabled ranking, oversized groups, exhausted limits, provider errors, refusals, timeouts, and invalid assessments leave the engine’s order and evidence available. Telemetry records the reason and provider-reported token usage when available. |

Local integration tests use a controlled ranker to verify validation, fallback, and persisted bilingual assessments. The adapter tests exercise the HTTP contract with a local server. [Ranking tests](../cmd/reconcilio/ranking_test.go) · [Adapter tests](../internal/openairank/client_test.go) · [Demo configuration](../../../README.md#optional-ai-ranking)
