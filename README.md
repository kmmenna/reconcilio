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
```
