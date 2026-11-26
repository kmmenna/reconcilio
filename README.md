# Reconcilio

Reconcilio helps Financial Operations Analysts reconcile internal ledger records with bank or card statements. The engine confirms pairs with strong evidence, sends competing Match Proposals to an analyst, and keeps discrepancies and past decisions visible. The workspace supports English and Brazilian Portuguese.

## Run locally

The local workflow supports Linux, macOS, and WSL2. Install mise and run `mise install` for the pinned tools. Have one of these container runtimes available before starting the stack.

- **Docker** is the default. Run Docker Engine or Docker Desktop with the Docker Compose plugin available as `docker compose`.
- **Podman** uses Docker Compose through `podman compose`. Run the Podman API service on Linux or WSL2, or a Podman machine on macOS. Install the Docker Compose CLI plugin or put the `docker-compose` executable on `PATH`. Set `PODMAN_COMPOSE_PROVIDER` to use a Docker Compose executable from another location.

Run the following from the worktree root. Compose starts PostgreSQL and runs the application and River queue migrations before starting the API and browser app.

```sh
mise run compose:up
mise exec -- node scripts/orchestration-env.mjs sh scripts/compose.sh exec backend reconcilio bootstrap-admin
```

Open [Reconcilio on localhost:3000](http://localhost:3000) and sign in as `admin` with the generated password. The bootstrap command prints the password only when creating the account. Save it for later sign-ins. The API generates a signing key at startup when `JWT_SECRET` is unset, so restarting the API then requires signing in again. Set a randomly generated secret of at least 32 bytes to keep existing tokens valid through a restart.

```sh
mise run compose:logs
mise run compose:stop
```

Stopping removes this project's containers and network. Its PostgreSQL volume is reused the next time you start the same project on the same engine.

### Worktree settings

Mise tasks read these settings from an optional `.env` in the worktree root. A local value takes precedence over an inherited shell value. Compose's automatic loading of the root file is disabled.

| Setting | Default | Purpose |
| --- | --- | --- |
| `CONTAINER_ENGINE` | `docker` | Set to `podman` to select Podman. An empty value selects Docker. |
| `COMPOSE_PROJECT_NAME` | Worktree directory name | Compose normalizes the default. Set a unique project name when directory names would produce the same project name. |
| `RECONCILIATION_API_PORT` | `8080` | API port published on `127.0.0.1`. |
| `ANALYST_WEB_PORT` | `3000` | Frontend port published on `127.0.0.1`. Its public origin defaults to `http://localhost` on this port. |

To run two worktrees side by side, put a different project name and different host ports in each worktree's root `.env`. For example, the first can use the defaults or specify `COMPOSE_PROJECT_NAME=reconcilio-main`, `RECONCILIATION_API_PORT=8080`, and `ANALYST_WEB_PORT=3000`. In the second, use:

```dotenv
COMPOSE_PROJECT_NAME=reconcilio-feature
RECONCILIATION_API_PORT=8081
ANALYST_WEB_PORT=3001
```

Run `mise run compose:up` in each worktree. The browser apps open on [localhost:3000](http://localhost:3000) and [localhost:3001](http://localhost:3001). Each project has its own PostgreSQL volume and needs its own initial administrator.

### Corporate Card walkthrough

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

### Local operations

The backend runs the API and River worker together by default. To run them in separate processes, set `RECONCILIO_WORKER_ENABLED=false` for the API and `RECONCILIO_API_ENABLED=false` for the worker. Run `reconcilio migrate` against the shared database before starting those processes.

If a rootless Podman setup needs a different OCI runtime for health checks or `exec`, start a separate Podman service in another terminal.

```sh
podman --runtime runc system service --time=0 "unix://$XDG_RUNTIME_DIR/reconcilio-podman.sock"
```

Set `DOCKER_HOST` to `unix://$XDG_RUNTIME_DIR/reconcilio-podman.sock` in the terminal running the mise tasks. Set `TESTCONTAINERS_RYUK_DISABLED=true` for the Go tests if rootless Podman prevents the Testcontainers cleanup service from starting. The test removes its PostgreSQL container on completion.

## Optional AI ranking

The Matching Engine runs the full walkthrough with the OpenAI variables unset. To enable bilingual proposal ranking, export `OPENAI_API_KEY` and `OPENAI_MODEL` in the shell that runs `mise run compose:up`. Use a model available to your OpenAI project that supports the Responses API and Structured Outputs.

On a fresh Compose project, bootstrap and sign in, import both Corporate Card files, and wait for processing. For existing imports with pending alternatives, open **Reconciliations**, select **Corporate Card**, and use **Reprocess account**. The worker reuses any assessment that already covers the current group. If you have already confirmed Canva, start a fresh worktree project with its own PostgreSQL volume and bootstrap it. Assign different host ports if the original stack remains up.

Open **Review queue** and **Compare alternatives** for `ERP-002`. Read the AI assessments and switch languages to see the same order and scores with translated explanations. After confirming Canva, follow **View reconciliation** to inspect the recorded assessment.

Each evaluation sends only the redacted descriptions for a complete group. The worker makes one attempt with a 15-second timeout. The default limits are five attempts per job and 50 per UTC day, including failed calls. Set `RECONCILIO_AI_PER_JOB_LIMIT` and `RECONCILIO_AI_DAILY_LIMIT` to override them. Both explanations are stored with one order and score, so the language selector can switch between them immediately.

## Console telemetry

The API and River worker emit JSON logs to stdout. Set `RECONCILIO_TRACE_STDOUT=true` and `RECONCILIO_METRIC_STDOUT=true` independently to export spans and metrics to the same console. Metrics are collected every 10 seconds.

To observe engine fallback locally, configure the ranking client with a test key and an endpoint that refuses connections inside the backend container.

```sh
RECONCILIO_TRACE_STDOUT=true RECONCILIO_METRIC_STDOUT=true \
  OPENAI_API_KEY=demo-only OPENAI_MODEL=demo-ranking \
  OPENAI_RESPONSES_URL=http://127.0.0.1:1/v1/responses \
  mise run compose:up
mise exec -- node scripts/orchestration-env.mjs sh scripts/compose.sh exec backend reconcilio bootstrap-admin
mise run compose:logs --no-log-prefix -f backend
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
