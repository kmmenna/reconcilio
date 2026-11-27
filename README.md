# Reconcilio

Reconcilio helps financial operations analysts reconcile internal ledger records with bank and card statements. Repeated amounts and overlapping source files make it time-consuming to identify records that describe the same payment. Reconcilio confirms unambiguous matches automatically and gives analysts the evidence to review proposals, investigate differences, and explain past decisions.

[Run locally](#run-locally) · [Try the demo](#corporate-card-walkthrough) · [Run automated tests](#checks-and-tests)

## What analysts can do

- **Import and validate CSVs.** Register Financial Accounts and Sources, preview files for one or more accounts, and inspect the inferred Import Coverage. Review conflicting reimports and choose whether to keep the original records or cancel the import.
- **Track reconciliation.** Follow background processing for each affected account and reprocess open Transactions on demand. The engine confirms a pair when both records have the same signed amount and currency and share a Payment Reference that appears once on each side of the same account.
- **Review competing proposals.** Compare external candidates and their evidence side by side, then confirm or reject a proposal. Optional AI ranking adds a score and explanations in English and Brazilian Portuguese.
- **Investigate differences and revisit decisions.** View open Transactions and known amount discrepancies. Open past decisions to see the original evidence and who made the decision.
- **Work in English or Brazilian Portuguese.** Choose an interface language or follow the browser preference. Local accounts identify analysts in the decision history.

## Corporate Card walkthrough

The demo uses synthetic ERP and card statement records to show an automatic match, a manual review, and an amount discrepancy. [Start Reconcilio locally](#run-locally).

1. Choose **English** from **Language** at sign-in or in the workspace to follow these button labels. **Português (Brasil)** switches the interface, dates, Money, and errors. **Automatic** returns to your browser preference. Shared links keep the same address and open in the signed-in reader’s language.
2. Sign in to open **Import CSV**. On narrow screens, select **Menu** to see the same destinations as the sidebar. Select **Demo ERP** as the Source and **Corporate Card** as the Financial Account. Choose [internal.csv](examples/demo/internal.csv) and select **Preview CSV**. Check the Source, side, account, and rows. The preview shows Import Coverage from November 16 through November 20, 2026. Select **Confirm import**. The receipt shows three Transactions imported. Open **Track processing** and wait for **Completed**.
3. Select **Import CSV** from the workspace navigation. Select **Demo Card Statement**, choose [external.csv](examples/demo/external.csv), and use the same account. Preview the file and confirm the import. Import Coverage runs from November 16 through November 21, 2026. The receipt shows four Transactions imported. Open **Track processing** and wait for **Completed**. The summary shows one automatic Reconciliation and two new Match Proposals.
4. From the completed job, open **View Reconciliations**, then **View details** for the automatic decision. Verify that `ERP-001` and `CARD-001` share the unique `DEMO-PAY-001` Payment Reference and BRL -89.90.
5. Open **Review queue** from the workspace navigation and select **Compare alternatives** for `ERP-002`. Canva (`CARD-002`) and Adobe (`CARD-003`) both have a signed amount of BRL -250.00 and a one-day difference from the internal record. Before deciding, open Adobe’s **View proposal** link in another tab or copy its address. Select **Confirm match** for Canva. Reload the saved Adobe link to see **Invalidated**, its original evidence, and a link to the Reconciliation that accepted Canva.
6. Select **Open Transactions** from the workspace navigation. Find `ERP-003` and `CARD-004`. Both share `DEMO-PAY-003` and remain open with an **Amount difference** of BRL 1.00.

See the [fixture reference](examples/demo/README.md) for the expected outcomes.

### Resolve a conflicting reimport

1. Open **Import CSV** and preview [reimport-amount-conflict.csv](examples/test-cases/reimport-amount-conflict.csv) with **Demo ERP** and **Corporate Card**. The preview shows Import Coverage from November 16 through November 22, 2026. Review the stored BRL -89.90 and incoming BRL -94.90 for `ERP-001`.
2. Select **Cancel import** first. Check **Import canceled** and the **Recorded decision**, then reload the receipt. **Open Transactions** has no `ERP-004`.
3. Upload the same fixture again and select **Keep originals and import**. Verify that the **Import confirmed** receipt records the decision and one imported Transaction. Find `ERP-004` at BRL -34.50 in **Open Transactions**. The original automatic Reconciliation still shows `ERP-001` with `CARD-001` at BRL -89.90. Uploading the unchanged fixture once more opens the confirmed receipt.

See the [conflict fixture reference](examples/test-cases/README.md) for both outcomes.

## Documentation

- Frontend architecture, component choices, and browser test coverage. [Analyst web README](apps/analyst-web/README.md)
- Go backend architecture, PostgreSQL jobs, and integration test coverage. [Reconciliation backend README](apps/reconciliation/README.md)
- How the backend ranks proposals, validates model responses, and falls back to engine evidence. [AI integration](apps/reconciliation/docs/AI_INTEGRATION.md)
- How Codex and its skills supported planning, implementation, and review. [AI usage](docs/AI_USAGE.md)
- Proposed product and operational improvements, including closing exports and manual matching of open Transactions. [Next steps](docs/NEXT_STEPS.md)
- The terminology used across the product and code. [Domain glossary](GLOSSARY.md)
- The rationale behind architectural and product decisions. [Architecture decisions](docs/adr/)
- The MVP approach, trade-offs, and verification. [Submission write-up](docs/submission.md)
- Compose, host development, and test settings. [Environment configuration](docs/environment.md)

## Run locally

The local workflow supports Linux, macOS, and WSL2. Install mise and run `mise install` for the pinned tools. Have one of these container runtimes available before starting the stack.

- **Docker** is the default. Run Docker Engine or Docker Desktop with the Docker Compose plugin available as `docker compose`.
- **Podman** uses Docker Compose through `podman compose`. Run the Podman API service on Linux or WSL2, or a Podman machine on macOS. Install the Docker Compose CLI plugin or put the `docker-compose` executable on `PATH`. Set `PODMAN_COMPOSE_PROVIDER` to use a Docker Compose executable from another location.

Run the following from the worktree root. The optional `env:init` task copies the three environment examples to their matching `.env` files and preserves existing files. Compose starts PostgreSQL and runs the application and River queue migrations before starting the API and browser app.

```sh
mise run env:init
mise run compose:up
mise run bootstrap-admin
```

Keep `RECONCILIO_API_ENABLED` enabled when using `mise run compose:up`. The browser app waits for the API health check before starting, and the task waits for the full stack to be ready.

Bootstrap requires a running, migrated database. With a running Compose backend, the task uses that container's database, including any explicit `DATABASE_URL`.

Open [Reconcilio on localhost:3000](http://localhost:3000) and sign in as `admin` with the generated password. The bootstrap command prints the password only when creating the account. Save it for later sign-ins. The API generates a signing key at startup when `JWT_SECRET` is unset, so restarting the API then requires signing in again. Set a randomly generated secret of at least 32 bytes to keep existing tokens valid through a restart.

```sh
mise run compose:logs
mise run compose:stop
```

Stopping removes this project's containers and network. Its PostgreSQL volume is reused the next time you start the same project on the same engine.

### Local operations

The backend runs the API and River worker together by default. To run them in separate processes, set `RECONCILIO_WORKER_ENABLED=false` for the API and `RECONCILIO_API_ENABLED=false` for the worker. Run `reconcilio migrate` against the shared database before starting those processes.

If a rootless Podman setup needs a different OCI runtime for health checks or `exec`, start a separate Podman service in another terminal.

```sh
podman --runtime runc system service --time=0 "unix://$XDG_RUNTIME_DIR/reconcilio-podman.sock"
```

Set `DOCKER_HOST` to `unix://$XDG_RUNTIME_DIR/reconcilio-podman.sock` in the terminal running the mise tasks. Set `TESTCONTAINERS_RYUK_DISABLED=true` for the Go tests if rootless Podman prevents the Testcontainers cleanup service from starting. The test removes its PostgreSQL container on completion.

## Optional AI ranking

The Matching Engine runs the full walkthrough with the OpenAI variables unset. To enable bilingual proposal ranking, set `OPENAI_API_KEY` and `OPENAI_MODEL` in `apps/reconciliation/.env` or export them in the shell that runs `mise run compose:up`. Use a model available to your OpenAI project that supports the Responses API and Structured Outputs.

On a fresh Compose project, bootstrap and sign in, import both Corporate Card files, and wait for processing. For existing imports with pending alternatives, open **Reconciliations**, select **Corporate Card**, and use **Reprocess account**. The worker reuses any assessment that already covers the current group. If you have already confirmed Canva, start a fresh worktree project with its own PostgreSQL volume and bootstrap it. Assign different host ports if the original stack remains up.

Open **Review queue** and **Compare alternatives** for `ERP-002`. Read the AI assessments and switch languages to see the same order and scores with translated explanations. After confirming Canva, follow **View reconciliation** to inspect the recorded assessment.

Each evaluation sends only the redacted descriptions for a complete group. The worker makes one attempt with a 15-second timeout. The default limits are five attempts per job and 50 per UTC day, including failed calls. Set `RECONCILIO_AI_PER_JOB_LIMIT` and `RECONCILIO_AI_DAILY_LIMIT` to override them. Both explanations are stored with one order and score, so the language selector can switch between them immediately.

## Console telemetry

The API and River worker emit JSON logs to stdout. Set `RECONCILIO_TRACE_STDOUT=true` and `RECONCILIO_METRIC_STDOUT=true` independently to export spans and metrics to the same console. Metrics are collected every 10 seconds. [Backend observability and available signals](apps/reconciliation/README.md#observability)

When an AI evaluation falls back with `fallback_reason: invalid_ranking`, look for `invalid_reason` on the same `AI ranking` record. For example, `en_explanation_missing` identifies a missing English explanation, and `candidate_count_mismatch` comes with the expected and returned counts. The event records fixed diagnostic codes and counts without candidate IDs, descriptions, explanations, or raw provider responses.

To observe engine fallback locally, configure the ranking client with a test key and an endpoint that refuses connections inside the backend container.

```sh
RECONCILIO_TRACE_STDOUT=true RECONCILIO_METRIC_STDOUT=true \
  OPENAI_API_KEY=demo-only OPENAI_MODEL=demo-ranking \
  OPENAI_RESPONSES_URL=http://127.0.0.1:1/v1/responses \
  mise run compose:up
mise run bootstrap-admin
mise run compose:logs --no-log-prefix -f backend
```

On a fresh demo database, preview and confirm the internal CSV, wait for its job to finish, then repeat for the external CSV. Find `HTTP request`, `Import operation`, and `Reconciliation job` records with the same `correlation_id`. The worker has the confirmation's `request_id`, the `import_id`, and the receipt's `job_id`. The external job emits `AI ranking` with `attempted: true` and `fallback_reason: provider_error` and still completes. Wait for a metric export and check `reconcilio.business.*` for seven imported Transactions, one automatic confirmation, and two internal and three external open Transactions. Replay the same CSV to verify the persisted totals stay the same. In a split deployment, each exporter reads the same database-wide business totals.

For live provider spend, select the OpenAI API project that owns the key. Open **Analytics → Spend**, choose the UTC date range, and filter by API key or model as needed. Recent calls can take time to appear. [OpenAI Spend dashboard help](https://help.openai.com/en/articles/10478918-reviewing-api-usage-and-costs#review-spend)

## Development

### Run the Go API on the host

Stop the Compose stack before switching to a host API on the same port. Run the development task from the worktree root.

```sh
mise run reconciliation:dev
```

The task starts the worktree's Compose PostgreSQL, waits for it to be ready, and applies the current checkout's application and River queue migrations before the API listens on the configured `RECONCILIATION_API_PORT`. Compose and host development share the PostgreSQL volume when they use the same engine and project with the default Compose connection. The backend's `DATABASE_URL` override applies to the Compose backend and migration step. The development task connects to the project's PostgreSQL service on its automatically assigned loopback port.

Once the API starts, run `mise run bootstrap-admin` in another terminal. With the Compose backend stopped, the task uses the worktree's running PostgreSQL. You can also bootstrap after stopping the host API while PostgreSQL is still running.

Stop the API with Ctrl+C. Run `mise run compose:stop` to stop the database and retain its data.

### Run the Next.js frontend on the host

Install the frontend dependencies, then start Next.js in its own terminal.

```sh
mise run web:install
mise run analyst-web:dev
```

Open the frontend at `http://localhost:<ANALYST_WEB_PORT>` (port 3000 by default). The frontend task starts independently. Sign-in and other API-backed features require a reachable Go API, which you can start in another terminal with `mise run reconciliation:dev`.

Set `BACKEND_API_ORIGIN` in `apps/analyst-web/.env` to tell Next.js which Go API to call. It defaults to `http://localhost:<RECONCILIATION_API_PORT>` for host development. Set `WEB_ORIGIN` in the same file if the browser opens the frontend at a different origin. Next.js uses that address to validate browser requests.

### Worktree settings

Mise tasks read these settings from an optional `.env` in the worktree root. A local value takes precedence over an inherited shell value. Compose's automatic loading of the root file is disabled. See [Environment configuration](docs/environment.md) for settings owned by the backend and frontend, including database initialization and origin overrides.

| Setting | Default | Purpose |
| --- | --- | --- |
| `CONTAINER_ENGINE` | `docker` | Set to `podman` to select Podman. An empty value selects Docker. |
| `COMPOSE_PROJECT_NAME` | Worktree directory name | Compose normalizes the default. Set a unique project name when directory names would produce the same project name. |
| `RECONCILIATION_API_PORT` | `8080` | API port on `127.0.0.1` in both modes. |
| `ANALYST_WEB_PORT` | `3000` | Frontend port published on `127.0.0.1`. Its public origin defaults to `http://localhost` on this port. |

To run two worktrees side by side, put a different project name and different host ports in each worktree's root `.env`. For example, the first can use the defaults or specify `COMPOSE_PROJECT_NAME=reconcilio-main`, `RECONCILIATION_API_PORT=8080`, and `ANALYST_WEB_PORT=3000`. In the second, use:

```dotenv
COMPOSE_PROJECT_NAME=reconcilio-feature
RECONCILIATION_API_PORT=8081
ANALYST_WEB_PORT=3001
```

Run `mise run compose:up` in each worktree. The browser apps open on [localhost:3000](http://localhost:3000) and [localhost:3001](http://localhost:3001). Each project has its own PostgreSQL volume and needs its own initial administrator.

To stay signed in to both instances in the same browser profile, open each at a different hostname. Session cookies are shared across ports on the same hostname. For example, keep `http://localhost:3000` for the first worktree, set `WEB_ORIGIN=http://127.0.0.1:3001` in the second worktree's `apps/analyst-web/.env`, and open [127.0.0.1:3001](http://127.0.0.1:3001) there.

### Checks and tests

Install the pinned tools with mise and install the web dependencies. `mise run test` runs the Go unit and PostgreSQL integration tests on the selected engine. Start Docker or the Podman API service or machine before running it.

```sh
mise install
mise run web:install
mise run check
mise run build
mise run test:environment
mise run test
```

`mise run test:environment` checks environment configuration and provider selection. It requires Docker Compose to generate the configuration, with no running container engine. To pass flags through to the Go suite, use `mise run test -- -run TestName`.

GitHub Actions runs the same build, lint, static analysis, PostgreSQL integration tests, and browser smoke tasks. Run `mise run db:generate` after changing module SQL or the schema. Install local Git hooks with `mise exec -- lefthook install`.

For the browser smoke tests, install Playwright Chromium after the web dependencies. Both variants use the selected engine and its Docker Compose provider. Each run creates its own Compose project, database credentials, and PostgreSQL volume, then removes its containers, network, volume, and build image tags when it exits. The smoke task chooses a free loopback frontend port by default. Set `RECONCILIO_SMOKE_PORT` to use a specific free port.

```sh
mise run browser:install
mise run smoke
RECONCILIO_SMOKE_RANKING=true mise run smoke
```

The first smoke run checks the engine-only workflow, both conflict receipts, localized validation, and review history. The second uses a controlled local ranker without an OpenAI key. It verifies the recorded evaluation, order, and score across languages in review and decision history, and checks that language and direct-link changes cause no additional provider requests. [Test environment details](docs/environment.md#tests)
