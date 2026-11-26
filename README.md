# Reconcilio

Reconcilio helps Financial Operations Analysts reconcile internal ledger records with bank or card statements. The engine confirms pairs with strong evidence, sends competing Match Proposals to an analyst, and keeps discrepancies and past decisions visible. The workspace supports English and Brazilian Portuguese.

## Run locally

From the checkout root, start the application with Docker and the Compose plugin. Compose starts PostgreSQL and runs the application and River queue migrations before starting the backend and browser app.

```sh
docker compose up --build -d --wait
docker compose exec backend reconcilio bootstrap-admin
```

Open [Reconcilio on localhost:3000](http://localhost:3000) and sign in as `admin` with the generated password. The bootstrap command prints the password only when creating the account. Save it for later sign-ins.

The demo binds the browser app to loopback. If you change `ANALYST_WEB_PORT`, set `WEB_ORIGIN` to the exact URL you open. The API generates a signing key at startup when `JWT_SECRET` is unset, so restarting the API then requires signing in again. Set a randomly generated secret of at least 32 bytes to keep existing tokens valid through a restart.

### Corporate Card walkthrough

1. Choose **English** from **Language** at sign-in or in the workspace to follow these button labels. **Português (Brasil)** switches the interface, dates, Money, and errors. **Automatic** returns to your browser preference. Shared links keep the same address and open in the signed-in reader’s language.
2. Open **Import CSV**. Select **Demo ERP** as the Source and **Corporate Card** as the Financial Account. Enter **11/16/2026** and **11/22/2026** as the start and end dates. In Portuguese those fields use **16/11/2026** and **22/11/2026**. Choose [internal.csv](examples/demo/internal.csv), select **Preview CSV**, check the Source, side, account, and rows, then select **Confirm import**. The receipt shows three Transactions imported. Open **Track processing** and wait for **Completed**.
3. Return to the workspace using **Reconcilio**, then open **Import CSV** again. Select **Demo Card Statement**, choose [external.csv](examples/demo/external.csv), and use the same account and dates. Preview and confirm. The receipt shows four Transactions imported. Open **Track processing** and wait for **Completed**. The summary shows one automatic Reconciliation and two new Match Proposals.
4. From the completed job, open **View Reconciliations**, then **View details** for the automatic decision. Verify that `ERP-001` and `CARD-001` share the unique `DEMO-PAY-001` Payment Reference and BRL -89.90.
5. Return to the workspace, open **Review queue**, and select **Compare alternatives** for `ERP-002`. Canva (`CARD-002`) and Adobe (`CARD-003`) both have a signed amount of BRL -250.00 and a one-day difference from the internal record. Before deciding, open Adobe’s **View proposal** link in another tab or copy its address. Select **Confirm match** for Canva. Reload the saved Adobe link to see **Invalidated**, its original evidence, and a link to the Reconciliation that accepted Canva.
6. Return to the workspace and open **Open Transactions**. Find `ERP-003` and `CARD-004`. Both share `DEMO-PAY-003` and remain open with an **Amount difference** of BRL 1.00.

See the [fixture reference](examples/demo/README.md) for the expected outcomes.

### Resolve a conflicting reimport

1. Open **Import CSV** and preview [reimport-amount-conflict.csv](examples/test-cases/reimport-amount-conflict.csv) with **Demo ERP**, **Corporate Card**, and the same coverage dates. Review the stored BRL -89.90 and incoming BRL -94.90 for `ERP-001`.
2. Select **Cancel import** first. Check **Import canceled** and the **Recorded decision**, then reload the receipt. **Open Transactions** has no `ERP-004`.
3. Upload the same fixture again and select **Keep originals and import**. Verify that the **Import confirmed** receipt records the decision and one imported Transaction. Find `ERP-004` at BRL -34.50 in **Open Transactions**. The original automatic Reconciliation still shows `ERP-001` with `CARD-001` at BRL -89.90. Uploading the unchanged fixture once more opens the confirmed receipt.

See the [conflict fixture reference](examples/test-cases/README.md) for both outcomes.

### Local operations

The backend runs the API and River worker together by default. To run them in separate processes, set `RECONCILIO_WORKER_ENABLED=false` for the API and `RECONCILIO_API_ENABLED=false` for the worker. Run `reconcilio migrate` against the shared database before starting those processes.

Stop the Compose project when you are done. Compose retains the database volume for your next session.

```sh
docker compose down
```

With a rootless Podman socket, point Docker Compose at it and use its compatibility builder.

```sh
export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"
export DOCKER_BUILDKIT=0
```

If the default OCI runtime denies container health checks or `exec`, start a separate Podman socket in another terminal.

```sh
podman --runtime runc system service --time=0 "unix://$XDG_RUNTIME_DIR/reconcilio-podman.sock"
```

Point `DOCKER_HOST` at `unix://$XDG_RUNTIME_DIR/reconcilio-podman.sock` in the terminal running Compose. Set `TESTCONTAINERS_RYUK_DISABLED=true` for the Go tests if rootless Podman prevents the Testcontainers cleanup service from starting. The test removes its PostgreSQL container on completion.

## Optional AI ranking

The Matching Engine runs the full walkthrough with the OpenAI variables unset. To enable bilingual proposal ranking, export `OPENAI_API_KEY` and `OPENAI_MODEL` in the shell that runs `docker compose up --build -d --wait`. Use a model available to your OpenAI project that supports the Responses API and Structured Outputs.

On a fresh Compose project, bootstrap and sign in, import both Corporate Card files, and wait for processing. For existing imports with pending alternatives, open **Reconciliations**, select **Corporate Card**, and use **Reprocess account**. The worker reuses any assessment that already covers the current group. If you have already confirmed Canva, use a fresh named project, for example `docker compose -p reconcilio-ranked up --build -d --wait`, with a separate volume. Stop the earlier project first to free port 3000 and use the same `-p reconcilio-ranked` for bootstrap and shutdown.

Open **Review queue** and **Compare alternatives** for `ERP-002`. Read the AI assessments and switch languages to see the same order and scores with translated explanations. After confirming Canva, follow **View reconciliation** to inspect the recorded assessment.

Each evaluation sends only the redacted descriptions for a complete group. The worker makes one attempt with a 15-second timeout. The default limits are five attempts per job and 50 per UTC day, including failed calls. Set `RECONCILIO_AI_PER_JOB_LIMIT` and `RECONCILIO_AI_DAILY_LIMIT` to override them. Both explanations are stored with one order and score, so the language selector can switch between them immediately.

## Console telemetry

The API and River worker emit JSON logs to stdout. Set `RECONCILIO_TRACE_STDOUT=true` and `RECONCILIO_METRIC_STDOUT=true` independently to export spans and metrics to the same console. Metrics are collected every 10 seconds.

To observe engine fallback locally, configure the ranking client with a test key and an endpoint that refuses connections inside the backend container.

```sh
RECONCILIO_TRACE_STDOUT=true RECONCILIO_METRIC_STDOUT=true \
  OPENAI_API_KEY=demo-only OPENAI_MODEL=demo-ranking \
  OPENAI_RESPONSES_URL=http://127.0.0.1:1/v1/responses \
  docker compose up --build -d --wait
docker compose exec backend reconcilio bootstrap-admin
docker compose logs --no-log-prefix -f backend
```

On a fresh demo database, preview and confirm the internal CSV, wait for its job to finish, then repeat for the external CSV. Find `HTTP request`, `Import operation`, and `Reconciliation job` records with the same `correlation_id`. The worker has the confirmation's `request_id`, the `import_id`, and the receipt's `job_id`. The external job emits `AI ranking` with `attempted: true` and `fallback_reason: provider_error` and still completes. Wait for a metric export and check `reconcilio.business.*` for seven imported Transactions, one automatic confirmation, and two internal and three external open Transactions. Replay the same CSV to verify the persisted totals stay the same. In a split deployment, each exporter reads the same database-wide business totals.

For live provider spend, select the OpenAI API project that owns the key. Open **Analytics → Spend**, choose the UTC date range, and filter by API key or model as needed. Recent calls can take time to appear. [OpenAI Spend dashboard help](https://help.openai.com/en/articles/10478918-reviewing-api-usage-and-costs#review-spend)

## Development

Install the pinned tools with mise and install the web dependencies. The Go integration test requires a Docker-compatible container runtime for Testcontainers.

```sh
mise install
mise run web:install
mise run check
mise run build
mise run test
```

GitHub Actions runs the same build, lint, static analysis, PostgreSQL integration tests, and browser smoke tasks. Run `mise run db:generate` after changing module SQL or the schema. Install local Git hooks with `mise exec -- lefthook install`.

For the browser smoke test, install Playwright Chromium and run the smoke task. It creates an isolated Compose project on port 3100 and removes its containers and test volume afterward. Set `RECONCILIO_SMOKE_PORT` if the port is occupied.

```sh
mise run browser:install
mise run smoke
RECONCILIO_SMOKE_RANKING=true mise run smoke
```

The first smoke run checks the engine-only workflow, both conflict receipts, localized validation, and review history. The second uses a controlled local ranker without an OpenAI key. It verifies the recorded evaluation, order, and score across languages in review and decision history, and checks that language and direct-link changes cause no additional provider requests.

Read the [submission write-up](docs/submission.md), [domain glossary](GLOSSARY.md), and [architecture decisions](docs/adr/) for the rationale behind the implementation.
