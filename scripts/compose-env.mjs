import { execve } from 'node:process';
import { loadBackendEnvironment, loadFrontendEnvironment, postgresURL } from './application-env.mjs';

const backend = loadBackendEnvironment();
const frontend = loadFrontendEnvironment({
  backendOrigin: 'http://backend:8080',
  webPort: process.env.ANALYST_WEB_PORT || '3000',
});
const env = {
  ...process.env,
  ...backend,
  ...frontend,
  DATABASE_URL: backend.DATABASE_URL || postgresURL(backend, 'postgres', 5432),
};

execve('/bin/sh', ['sh', '-c', 'exec "$@"', 'reconcilio', ...process.argv.slice(2)], env);
