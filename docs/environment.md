# Environment configuration

Run `mise run env:init` from the worktree root to copy each `.env.example` to its matching `.env` when that file is missing. Existing files are preserved. The examples list optional overrides as comments, so an initialized checkout still accepts shell settings and built-in defaults. A fresh checkout works with all three files missing.

For each supported setting, mise tasks use the value in its owning file, then the inherited shell value, then the runtime default. An explicitly empty value takes precedence over the shell. For ports, origins, PostgreSQL initialization settings, and `DATABASE_URL`, an empty value selects the default. For credentials such as `OPENAI_API_KEY`, an empty value clears the credential. Quote values containing `#` or significant whitespace. Dollar signs in these files are literal.

| File | Settings |
| --- | --- |
| `/.env` | `CONTAINER_ENGINE`, `COMPOSE_PROJECT_NAME`, `RECONCILIATION_API_PORT`, `ANALYST_WEB_PORT` |
| `apps/reconciliation/.env` | Database settings, `JWT_SECRET`, runtime component flags, telemetry settings, and AI ranking settings |
| `apps/analyst-web/.env` | `BACKEND_API_ORIGIN` and `WEB_ORIGIN` |

The root file configures worktree orchestration. The application files configure their respective services at startup and are excluded from container image builds. See the [backend example](../apps/reconciliation/.env.example) and [frontend example](../apps/analyst-web/.env.example) for the supported keys.

## Database

The Compose database uses `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD` from the backend settings to initialize a new PostgreSQL volume. Each defaults to `reconcilio` for the local demo. Once the volume is initialized, update existing databases and roles in PostgreSQL before changing their connection settings. `mise run compose:stop` retains the project's volume.

The Compose API and migration step both use `DATABASE_URL` when configured. With the default URL, both connect to the `postgres` service using the configured database, role, and password. The migration step applies the application and River queue migrations to its connection target before the API starts.

In both modes, Compose publishes PostgreSQL on a dynamically assigned port on `127.0.0.1`. `mise run reconciliation:dev` waits for PostgreSQL and combines its published port with the backend's PostgreSQL initialization settings to connect from the host. The task uses that connection for the current checkout's Go migration command and API, even when `DATABASE_URL` is set for Compose. With the default Compose connection, switching modes on the same engine and project reuses the same named volume.

## Frontend origins

`BACKEND_API_ORIGIN` is the Go API URL used by the Next.js server. It defaults to `http://backend:8080` under Compose, including when the API is published on a custom host port. With `mise run analyst-web:dev`, it defaults to `http://localhost:<RECONCILIATION_API_PORT>`. An explicit value in `apps/analyst-web/.env` applies in both modes.

`WEB_ORIGIN` is the address used by the browser. It defaults to `http://localhost` on the configured `ANALYST_WEB_PORT`. Set it to the browser-facing scheme, hostname, and port when opening the app from another origin, such as `http://127.0.0.1:3001`. The frontend checks that browser login and other writes originate at that address.

## Tests

`mise run test` runs the Go suite and starts PostgreSQL through Testcontainers. It uses the active Docker CLI endpoint for `CONTAINER_ENGINE=docker` and the Podman Compose-compatible endpoint for `CONTAINER_ENGINE=podman`. Start the selected daemon, API service, or machine first. With a custom Podman socket, export its `unix://` URL as `DOCKER_HOST`. Clear any conflicting `tc.host` in `~/.testcontainers.properties` so Testcontainers uses the selected endpoint. If Ryuk cannot access the Podman socket, set `TESTCONTAINERS_RYUK_DISABLED=true`. The tests each register cleanup for their PostgreSQL containers.

Install web dependencies with `mise run web:install` and Playwright Chromium with `mise run browser:install`. Run `mise run smoke` for the browser workflow or `RECONCILIO_SMOKE_RANKING=true mise run smoke` for the controlled ranking workflow. Both use Docker Compose on the selected engine.

Smoke tests run in their own Compose project with disposable PostgreSQL credentials, a separate volume, and test ports. The API port is dynamically assigned. The frontend uses a free loopback port or the `RECONCILIO_SMOKE_PORT` override. Smoke uses its own service settings and the ranking variant uses a local provider. Each run removes its containers, network, and database volume when it exits, so the worktree stack and its data remain available.
