import { execve } from 'node:process';
import { loadFrontendEnvironment } from './application-env.mjs';

const webPort = process.env.ANALYST_WEB_PORT;
const env = {
  ...process.env,
  ...loadFrontendEnvironment({
    backendOrigin: `http://localhost:${process.env.RECONCILIATION_API_PORT}`,
    webPort,
  }),
};

execve('/bin/sh', [
  'sh', '-c', 'exec "$@"', 'reconcilio',
  'npm', 'run', 'dev', '--', '--hostname', '127.0.0.1', '--port', webPort,
], env);
