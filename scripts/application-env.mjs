import { readFileSync } from 'node:fs';
import { parseEnv } from 'node:util';

const backendSettings = [
  'DATABASE_URL',
  'POSTGRES_DB',
  'POSTGRES_USER',
  'POSTGRES_PASSWORD',
  'JWT_SECRET',
  'RECONCILIO_API_ENABLED',
  'RECONCILIO_WORKER_ENABLED',
  'RECONCILIO_TRACE_STDOUT',
  'RECONCILIO_METRIC_STDOUT',
  'OPENAI_API_KEY',
  'OPENAI_MODEL',
  'OPENAI_RESPONSES_URL',
  'RECONCILIO_AI_PER_JOB_LIMIT',
  'RECONCILIO_AI_DAILY_LIMIT',
];

function readApplicationEnvironment(app, settings, inherited) {
  let local = {};
  try {
    local = parseEnv(readFileSync(new URL(`../apps/${app}/.env`, import.meta.url), 'utf8'));
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }

  const environment = {};
  for (const name of settings) {
    const value = local[name] ?? inherited[name];
    if (value !== undefined) environment[name] = value;
  }
  return environment;
}

export function loadBackendEnvironment(inherited = process.env) {
  const environment = readApplicationEnvironment('reconciliation', backendSettings, inherited);
  environment.POSTGRES_DB ||= 'reconcilio';
  environment.POSTGRES_USER ||= 'reconcilio';
  environment.POSTGRES_PASSWORD ||= 'reconcilio';
  return environment;
}

export function postgresURL(environment, host, port) {
  const user = encodeURIComponent(environment.POSTGRES_USER);
  const password = encodeURIComponent(environment.POSTGRES_PASSWORD);
  const database = encodeURIComponent(environment.POSTGRES_DB);
  return `postgres://${user}:${password}@${host}:${port}/${database}?sslmode=disable`;
}

export function loadFrontendEnvironment({ backendOrigin, webPort }, inherited = process.env) {
  const environment = readApplicationEnvironment('analyst-web', ['BACKEND_API_ORIGIN', 'WEB_ORIGIN'], inherited);
  return {
    BACKEND_API_ORIGIN: environment.BACKEND_API_ORIGIN || backendOrigin,
    WEB_ORIGIN: environment.WEB_ORIGIN || `http://localhost:${webPort}`,
  };
}
