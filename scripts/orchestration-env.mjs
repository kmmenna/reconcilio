import { readFileSync } from 'node:fs';
import { execve } from 'node:process';
import { parseEnv } from 'node:util';

const worktree = new URL('../', import.meta.url);
const defaults = {
  CONTAINER_ENGINE: 'docker',
  COMPOSE_PROJECT_NAME: '',
  RECONCILIATION_API_PORT: '8080',
  ANALYST_WEB_PORT: '3000',
};

let local = {};
try {
  local = parseEnv(readFileSync(new URL('.env', worktree), 'utf8'));
} catch (error) {
  if (error.code !== 'ENOENT') throw error;
}

const env = { ...process.env };
for (const [name, fallback] of Object.entries(defaults)) {
  env[name] = (local[name] ?? env[name]) || fallback;
}

// Let Compose derive and normalize its project name from the worktree directory.
if (!env.COMPOSE_PROJECT_NAME) delete env.COMPOSE_PROJECT_NAME;

execve('/bin/sh', ['sh', '-c', 'exec "$@"', 'reconcilio', ...process.argv.slice(2)], env);
