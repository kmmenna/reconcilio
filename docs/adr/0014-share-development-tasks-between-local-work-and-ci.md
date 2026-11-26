# Share development tasks between local work and CI

Use mise to pin development tool versions and define tasks for the Go backend and React frontend. Use Lefthook for local Git hooks and GitHub Actions for CI. Both invoke the shared mise tasks so developers can reproduce the same builds and checks locally.

Use a root `mise.toml` to orchestrate both projects in the MVP. Define each task's working directory and its dependencies in mise. Revisit dedicated monorepo orchestration when cross-project dependencies or CI runtime justify affected-project execution or shared build caching.

Use mise tasks to start, inspect, and stop the local Compose stack. The optional root `.env` configures the container engine, project name, and published API and frontend ports. Local values take precedence over the shell, with defaults for a fresh worktree. Mise passes those settings to Docker Compose and disables the root file's automatic loading by Compose. The Podman path uses the same Docker Compose provider through `podman compose`.

Compose derives the default project name from the worktree directory. Distinct projects on the same engine use independent named PostgreSQL volumes. Bringing a project down retains its volume so the next start can reuse its data.
