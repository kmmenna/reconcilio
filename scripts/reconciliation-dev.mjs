import { execve } from 'node:process';
import { loadBackendEnvironment } from './application-env.mjs';
import { loadHostBackendEnvironment } from './host-backend-env.mjs';

try {
  const backend = loadBackendEnvironment();
  if (backend.DATABASE_URL) {
    console.error("Development uses the worktree's Compose PostgreSQL. DATABASE_URL applies to Compose backend and migrations.");
  }
  const env = loadHostBackendEnvironment(backend);
  env.HTTP_ADDR = `127.0.0.1:${process.env.RECONCILIATION_API_PORT}`;
  execve('/bin/sh', ['sh', '-c', 'cd apps/reconciliation && go run ./cmd/reconcilio migrate && exec go run ./cmd/reconcilio serve'], env);
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
