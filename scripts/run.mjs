#!/usr/bin/env node
// Cross-platform task runner (no make on Windows).
//   pnpm dev     API (Demo mode) + all Vite dev servers; seeds the clean start if the database is empty
//   pnpm seed    rebuild the demo database as the clean start; `pnpm seed --story` adds the demo story
//                (stop the API first, or use Admin → Pengaturan → Reset demo while it runs)
// The API, seed and tests need POSTGRE_URL (ADR-0007), from the shell or the repo-root .env
// (see .env.example); a variable already set in the shell wins over .env.
//   pnpm demo    build everything, run API + Caddy with LAN HTTPS
//   pnpm gen     regenerate sqlc code, API types and design tokens
import { spawn, spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { networkInterfaces } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const api = join(root, 'api');
const tools = join(root, '.tools');
const exe = process.platform === 'win32' ? '.exe' : '';
if (existsSync(join(root, '.env'))) process.loadEnvFile(join(root, '.env'));
const demoEnv = { SL_DEMO_MODE: 'true', SL_DATA_DIR: join(root, 'data'), SL_ASSETS_DIR: join(root, 'docs', 'images') };

// Windows needs a shell to resolve pnpm.cmd; pass one quoted command line rather than args.
const quote = (s) => (/[\s"]/.test(s) ? `"${s.replaceAll('"', '\\"')}"` : s);
const commandLine = (cmd, args) => [cmd, ...args].map(quote).join(' ');

function run(cmd, args, opts = {}) {
  const r = spawnSync(commandLine(cmd, args), { stdio: 'inherit', shell: true, ...opts });
  if (r.status !== 0) process.exit(r.status ?? 1);
}

function start(name, cmd, args, opts = {}) {
  const child = spawn(commandLine(cmd, args), { stdio: ['ignore', 'pipe', 'pipe'], shell: true, ...opts });
  const prefix = (line) => line && process.stdout.write(`[${name}] ${line}\n`);
  for (const stream of [child.stdout, child.stderr]) {
    let buf = '';
    stream.on('data', (d) => {
      buf += d;
      const lines = buf.split(/\r?\n/);
      buf = lines.pop();
      lines.forEach(prefix);
    });
  }
  child.on('exit', (code) => {
    prefix(`exited (${code})`);
    shutdown(code ?? 0);
  });
  children.push(child);
  return child;
}

const children = [];
function shutdown(code = 0) {
  for (const c of children) {
    if (c.exitCode === null) {
      if (process.platform === 'win32') spawnSync('taskkill', ['/pid', String(c.pid), '/T', '/F'], { stdio: 'ignore' });
      else c.kill('SIGTERM');
    }
  }
  process.exit(code);
}
process.on('SIGINT', () => shutdown(0));

function lanAddress() {
  for (const ifaces of Object.values(networkInterfaces())) {
    for (const i of ifaces ?? []) if (i.family === 'IPv4' && !i.internal) return i.address;
  }
  return '127.0.0.1';
}

function gen() {
  const sqlc = join(tools, 'sqlc', `sqlc${exe}`);
  if (!existsSync(sqlc)) {
    console.error('sqlc missing: run  powershell -ExecutionPolicy Bypass -File tools/fetch-tools.ps1');
    process.exit(1);
  }
  run(sqlc, ['generate'], { cwd: api });
  run('pnpm', ['--filter', '@sushi-latte/api-client', 'gen'], { cwd: root });
  run('pnpm', ['--filter', '@sushi-latte/ui', 'tokens'], { cwd: root });
}

function seed(args = []) {
  run('go', ['run', './cmd/seed', ...args], { cwd: api, env: { ...process.env, ...demoEnv } });
}

function needDatabase() {
  if (process.env.POSTGRE_URL) return;
  console.error('POSTGRE_URL is not set. Copy .env.example to .env and point it at a PostgreSQL database:\n' +
    '  POSTGRE_URL=postgres://postgres:<password>@localhost:5432/sushi_latte?sslmode=disable');
  process.exit(1);
}

const task = process.argv[2];
if (['seed', 'dev', 'demo', 'test-api'].includes(task)) needDatabase();
switch (task) {
  case 'gen':
    gen();
    break;
  case 'seed':
    seed(process.argv.slice(3).map((a) => (a === '--story' ? '-story' : a)));
    break;
  case 'test-api':
    run('go', ['test', './...'], { cwd: api });
    break;
  case 'dev': {
    run('pnpm', ['--filter', '@sushi-latte/api-client', 'gen'], { cwd: root });
    run('pnpm', ['--filter', '@sushi-latte/ui', 'tokens'], { cwd: root });
    seed(['-if-empty']);
    start('api', 'go', ['run', './cmd/server'], { cwd: api, env: { ...process.env, ...demoEnv } });
    start('web', 'pnpm', ['exec', 'turbo', 'run', 'dev', '--filter=./web/apps/*', '--ui=stream'], { cwd: root });
    console.log('\n  Client Menu   http://localhost:5173/\n  Staff         http://localhost:5174/staff/\n  Display       http://localhost:5175/display/\n');
    break;
  }
  case 'demo': {
    const caddy = join(tools, 'caddy', `caddy${exe}`);
    if (!existsSync(caddy)) {
      console.error('caddy missing: run  powershell -ExecutionPolicy Bypass -File tools/fetch-tools.ps1');
      process.exit(1);
    }
    run('pnpm', ['exec', 'turbo', 'run', 'build', '--filter=./web/apps/*'], { cwd: root });
    run('go', ['build', '-o', join(tools, `sl-api${exe}`), './cmd/server'], { cwd: api });
    seed(['-if-empty']);
    const ip = lanAddress();
    start('api', join(tools, `sl-api${exe}`), [], { cwd: api, env: { ...process.env, ...demoEnv } });
    start('caddy', caddy, ['run', '--config', join(root, 'deploy', 'Caddyfile'), '--adapter', 'caddyfile'], {
      cwd: root,
      env: { ...process.env, SL_LAN_IP: ip, SL_ROOT: root },
    });
    console.log(`\n  Install the demo CA on each device once:  http://${ip}:8081/ca.crt`);
    console.log(`  Client Menu   https://${ip}:8443/\n  Staff         https://${ip}:8443/staff/\n  Display       https://${ip}:8443/display/\n`);
    break;
  }
  default:
    console.error('usage: node scripts/run.mjs <dev|seed|demo|gen|test-api>');
    process.exit(1);
}
