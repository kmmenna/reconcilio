# Share development tasks between local work and CI

Use mise to pin development tool versions and define tasks for the Go backend and React frontend. Use Lefthook for local Git hooks and GitHub Actions for CI. Both invoke the shared mise tasks so developers can reproduce the same builds and checks locally.

Use a root `mise.toml` to orchestrate both projects in the MVP. Define each task's working directory and its dependencies in mise. Revisit dedicated monorepo orchestration when cross-project dependencies or CI runtime justify affected-project execution or shared build caching.
