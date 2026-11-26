import { spawnSync } from 'node:child_process';
import { loadBackendEnvironment, postgresURL } from './application-env.mjs';

export function loadHostBackendEnvironment(backend = loadBackendEnvironment()) {
  const env = { ...process.env, ...backend };
  const result = spawnSync('sh', ['scripts/compose.sh', 'port', 'postgres', '5432'], {
    env,
    encoding: 'utf8',
    stdio: ['inherit', 'pipe', 'inherit'],
  });
  if (result.error) throw result.error;

  const port = result.stdout?.trim().match(/^127\.0\.0\.1:([1-9]\d*)$/)?.[1];
  if (result.status !== 0 || !port) {
    throw new Error("Could not find the worktree's published PostgreSQL port. Start the project's PostgreSQL service first.");
  }
  return { ...env, DATABASE_URL: postgresURL(backend, '127.0.0.1', port) };
}
