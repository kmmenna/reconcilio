# Reconcilio

Reconcilio is a workspace for financial transaction reconciliation. Sign in to preview financial records from a CSV before importing Transactions.

## Run locally

Start the application and its database with Docker Compose. The migration step prepares PostgreSQL before the API starts.

```sh
docker compose up --build -d --wait
docker compose exec backend reconcilio bootstrap-admin
```

Open [http://localhost:3000](http://localhost:3000) and sign in as `admin` with the generated password. The bootstrap command prints that password only when creating the account. Preserve it for later sign-ins.

To try the preview workflow, open **Import CSV** in the workspace and use the [Corporate Card demo files](examples/demo/README.md).

The demo binds the browser app to loopback. If you change `ANALYST_WEB_PORT`, set `WEB_ORIGIN` to the exact URL you open. The API generates a signing key at startup when `JWT_SECRET` is unset, so restarting the API then requires signing in again. Set a randomly generated secret of at least 32 bytes to keep existing tokens valid through a restart.

The backend runs the API and River worker together by default. In a separate process, set `RECONCILIO_WORKER_ENABLED=false` for the API or `RECONCILIO_API_ENABLED=false` for the worker. Run `reconcilio migrate` against the shared database first to apply both application and River migrations. Compose does this at startup.

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

The Matching Engine can run the full demo on its own. To enable bilingual proposal ranking, set `OPENAI_API_KEY` and `OPENAI_MODEL` in the backend environment before starting Compose. Choose an OpenAI model that supports the Responses API and Structured Outputs, such as `gpt-5.4-mini` if your project has access. Then import the Corporate Card demo files and open **Review queue**. For an existing import, use **Reprocess account** on its Financial Account. The worker reuses any assessment that already covers the current group.

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

Run `mise run db:generate` after changing module SQL or the schema. Install local Git hooks with `mise exec -- lefthook install`.

For the browser smoke test, install Playwright Chromium and run the smoke task. It creates an isolated Compose project on port 3100 and removes its containers and test volume afterward. Set `RECONCILIO_SMOKE_PORT` if the port is occupied.

```sh
mise run browser:install
mise run smoke
RECONCILIO_SMOKE_RANKING=true mise run smoke
```

The second command checks ranked review and decision history with a local test provider.
