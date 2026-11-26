import { readFileSync, writeFileSync } from 'node:fs';

const worktree = new URL('../', import.meta.url);

for (const file of ['.env', 'apps/reconciliation/.env', 'apps/analyst-web/.env']) {
  const example = readFileSync(new URL(`${file}.example`, worktree));
  try {
    writeFileSync(new URL(file, worktree), example, { flag: 'wx', mode: 0o600 });
  } catch (error) {
    if (error.code !== 'EEXIST') throw error;
  }
}
