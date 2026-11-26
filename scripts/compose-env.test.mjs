import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { closeSync, copyFileSync, cpSync, mkdirSync, mkdtempSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const appDirs = ['apps/reconciliation', 'apps/analyst-web'];

function worktree(t) {
  const directory = mkdtempSync(join(tmpdir(), 'reconcilio-config-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  for (const app of appDirs) mkdirSync(join(directory, app), { recursive: true });
  for (const file of ['mise.toml', 'compose.yaml']) copyFileSync(join(root, file), join(directory, file));
  cpSync(join(root, 'scripts'), join(directory, 'scripts'), { recursive: true });
  return directory;
}

function runTask(directory, task, env = {}) {
  const log = join(directory, 'task.log');
  const fd = openSync(log, 'w');
  let result;
  try {
    result = spawnSync('mise', ['run', task], {
      cwd: directory,
      env: { ...process.env, MISE_TRUSTED_CONFIG_PATHS: directory, MISE_OFFLINE: '1', ...env },
      stdio: ['ignore', fd, fd],
    });
  } finally {
    closeSync(fd);
  }
  assert.ifError(result.error);
  assert.equal(result.status, 0, readFileSync(log, 'utf8'));
}

test('env:init copies each example only when its local file is missing', (t) => {
  const directory = worktree(t);
  const envFiles = ['.env', ...appDirs.map((app) => join(app, '.env'))];
  for (const file of envFiles) {
    const example = `${file}.example`;
    copyFileSync(join(root, example), join(directory, example));
  }
  const existing = join(directory, 'apps/reconciliation/.env');
  writeFileSync(existing, 'POSTGRES_DB=existing_database\n');
  runTask(directory, 'env:init');

  for (const file of envFiles.filter((file) => join(directory, file) !== existing)) {
    assert.equal(
      readFileSync(join(directory, file), 'utf8'),
      readFileSync(join(directory, `${file}.example`), 'utf8'),
    );
  }
  assert.equal(readFileSync(existing, 'utf8'), 'POSTGRES_DB=existing_database\n');

  const rootEnv = join(directory, '.env');
  writeFileSync(rootEnv, 'ANALYST_WEB_PORT=3101\n');
  runTask(directory, 'env:init');
  assert.equal(readFileSync(rootEnv, 'utf8'), 'ANALYST_WEB_PORT=3101\n');
});

function composeConfig(directory, env = {}) {
  const bin = join(directory, 'bin');
  mkdirSync(bin);
  writeFileSync(join(bin, 'docker'), [
    '#!/bin/sh',
    'set -eu',
    '[ "$*" = "compose up --build -d --wait" ]',
    'export PATH="$TEST_DOCKER_PATH"',
    'if docker compose version >/dev/null 2>&1; then',
    '  exec docker compose config --format json > "$TEST_CONFIG"',
    'fi',
    'if [ -n "${PODMAN_COMPOSE_PROVIDER:-}" ]; then',
    '  exec "$PODMAN_COMPOSE_PROVIDER" config --format json > "$TEST_CONFIG"',
    'fi',
    'exec docker-compose config --format json > "$TEST_CONFIG"',
  ].join('\n'), { mode: 0o700 });
  const config = join(directory, 'config.json');
  runTask(directory, 'compose:up', {
    CONTAINER_ENGINE: 'docker',
    COMPOSE_PROJECT_NAME: '',
    ANALYST_WEB_PORT: '',
    RECONCILIATION_API_PORT: '',
    DATABASE_URL: '',
    POSTGRES_DB: '',
    POSTGRES_USER: '',
    POSTGRES_PASSWORD: '',
    BACKEND_API_ORIGIN: '',
    WEB_ORIGIN: '',
    OPENAI_RESPONSES_URL: '',
    ...env,
    TEST_DOCKER_PATH: process.env.PATH,
    TEST_CONFIG: config,
    PATH: `${bin}:${process.env.PATH}`,
  });
  return JSON.parse(readFileSync(config, 'utf8')).services;
}

test('Compose defaults work without environment files', (t) => {
  const services = composeConfig(worktree(t), { RECONCILIATION_API_PORT: '8081', ANALYST_WEB_PORT: '3101' });
  const db = services.postgres;
  assert.deepEqual(db.environment, {
    POSTGRES_DB: 'reconcilio',
    POSTGRES_USER: 'reconcilio',
    POSTGRES_PASSWORD: 'reconcilio',
  });
  assert.equal(services.backend.ports[0].published, '8081');
  assert.equal(services.backend.environment.DATABASE_URL, 'postgres://reconcilio:reconcilio@postgres:5432/reconcilio?sslmode=disable');
  assert.equal(services.migrate.environment.DATABASE_URL, services.backend.environment.DATABASE_URL);
  assert.equal(services['analyst-web'].environment.BACKEND_API_ORIGIN, 'http://backend:8080');
  assert.equal(services['analyst-web'].environment.WEB_ORIGIN, 'http://localhost:3101');
});

test('Compose uses application values before shell fallback and scopes settings to their owner', (t) => {
  const directory = worktree(t);
  writeFileSync(join(directory, '.env'), [
    'BACKEND_API_ORIGIN=http://ignored-root:8080',
    'JWT_SECRET=ignored-root',
    'OPENAI_RESPONSES_URL=http://ignored-root:9001',
  ].join('\n'));
  writeFileSync(join(directory, 'apps/reconciliation/.env'), [
    'POSTGRES_DB=local/database',
    'POSTGRES_USER=local@user',
    'POSTGRES_PASSWORD="local/@#$ password"',
    'JWT_SECRET=from-backend-file',
    'OPENAI_API_KEY=',
    'RECONCILIO_WORKER_ENABLED=false',
    'WEB_ORIGIN=http://ignored-backend:3000',
  ].join('\n'));
  writeFileSync(join(directory, 'apps/analyst-web/.env'), [
    'BACKEND_API_ORIGIN=http://override-backend:9000',
    'JWT_SECRET=ignored-frontend',
  ].join('\n'));

  const services = composeConfig(directory, {
    POSTGRES_DB: 'inherited_db',
    JWT_SECRET: 'inherited-secret',
    OPENAI_API_KEY: 'inherited-key',
    OPENAI_MODEL: 'inherited-model',
    RECONCILIO_WORKER_ENABLED: 'true',
    BACKEND_API_ORIGIN: 'http://inherited-backend:8080',
    WEB_ORIGIN: 'http://inherited-frontend:3100',
  });
  assert.equal(services.postgres.environment.POSTGRES_DB, 'local/database');
  assert.equal(services.postgres.environment.POSTGRES_USER, 'local@user');
  const backend = services.backend.environment;
  assert.equal(backend.DATABASE_URL, 'postgres://local%40user:local%2F%40%23%24%20password@postgres:5432/local%2Fdatabase?sslmode=disable');
  assert.equal(backend.JWT_SECRET, 'from-backend-file');
  assert.equal(backend.OPENAI_API_KEY, '');
  assert.equal(backend.OPENAI_MODEL, 'inherited-model');
  assert.equal(backend.OPENAI_RESPONSES_URL, '');
  assert.equal(backend.RECONCILIO_WORKER_ENABLED, 'false');
  assert.equal(backend.WEB_ORIGIN, undefined);
  const web = services['analyst-web'].environment;
  assert.equal(web.BACKEND_API_ORIGIN, 'http://override-backend:9000');
  assert.equal(web.WEB_ORIGIN, 'http://inherited-frontend:3100');
  assert.equal(web.JWT_SECRET, undefined);
});

test('an explicit database URL goes to both migration and API', (t) => {
  const directory = worktree(t);
  const url = 'postgres://demo:demo@other-postgres:5432/override?sslmode=require';
  writeFileSync(join(directory, 'apps/reconciliation/.env'), `DATABASE_URL=${url}\n`);
  writeFileSync(join(directory, 'apps/analyst-web/.env'), 'WEB_ORIGIN=http://127.0.0.1:3101\n');

  const services = composeConfig(directory, {
    DATABASE_URL: 'postgres://ignored/override',
    WEB_ORIGIN: 'http://ignored:3101',
  });
  assert.equal(services.backend.environment.DATABASE_URL, url);
  assert.equal(services.migrate.environment.DATABASE_URL, url);
  assert.equal(services['analyst-web'].environment.WEB_ORIGIN, 'http://127.0.0.1:3101');
});

test('Podman falls back to the Docker CLI plugin when docker-compose on PATH is legacy', (t) => {
  const directory = worktree(t);
  const bin = join(directory, 'bin');
  const plugin = join(directory, 'cli plugins', 'docker-compose');
  const commandLog = join(directory, 'compose-command.log');
  const executable = { mode: 0o700 };
  mkdirSync(bin);
  mkdirSync(join(directory, 'cli plugins'));
  writeFileSync(join(bin, 'docker-compose'), [
    '#!/bin/sh',
    '[ "$1" = version ] || exit 1',
    'printf "%s\\n" "docker-compose version 1.29.2, build unknown"',
  ].join('\n'), executable);
  writeFileSync(join(bin, 'docker'), [
    '#!/bin/sh',
    '[ "$1" = info ] || exit 1',
    'printf "%s\\n" "$TEST_COMPOSE_PLUGIN"',
  ].join('\n'), executable);
  writeFileSync(join(bin, 'podman'), [
    '#!/bin/sh',
    '[ "$1" = compose ] || exit 1',
    'shift',
    'exec "$PODMAN_COMPOSE_PROVIDER" "$@"',
  ].join('\n'), executable);
  writeFileSync(plugin, [
    '#!/bin/sh',
    'if [ "$1" = version ]; then',
    '  printf "%s\\n" "Docker Compose version v5.8.0"',
    'else',
    '  printf "%s\\n" "$@" > "$TEST_COMPOSE_COMMAND_LOG"',
    'fi',
  ].join('\n'), executable);

  runTask(directory, 'compose:logs', {
    CONTAINER_ENGINE: 'podman',
    PODMAN_COMPOSE_PROVIDER: '',
    TEST_COMPOSE_PLUGIN: plugin,
    TEST_COMPOSE_COMMAND_LOG: commandLog,
    PATH: `${bin}:${process.env.PATH}`,
  });
  assert.equal(readFileSync(commandLog, 'utf8'), 'logs\n');
});
