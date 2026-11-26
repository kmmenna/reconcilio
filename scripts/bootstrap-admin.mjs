import { spawnSync } from 'node:child_process';
import { execve } from 'node:process';
import { loadHostBackendEnvironment } from './host-backend-env.mjs';

try {
  const backend = spawnSync('sh', ['scripts/compose.sh', 'ps', '--quiet', '--status', 'running', 'backend'], {
    encoding: 'utf8',
    stdio: ['inherit', 'pipe', 'inherit'],
  });
  if (backend.error) throw backend.error;
  if (backend.status !== 0) throw new Error("Could not check the worktree's Compose backend.");

  if (backend.stdout.trim()) {
    execve('/bin/sh', ['sh', 'scripts/compose.sh', 'exec', '-T', 'backend', 'reconcilio', 'bootstrap-admin'], process.env);
  } else {
    const env = loadHostBackendEnvironment();
    execve('/bin/sh', ['sh', '-c', 'cd apps/reconciliation && exec go run ./cmd/reconcilio bootstrap-admin'], env);
  }
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
