#!/usr/bin/env node
// Cross-platform task runner (no make on Windows).
//   pnpm dev     API (Demo mode) + all Vite dev servers
//   pnpm seed    rebuild the demo database (stop the API first)
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

function seed() {
  run('go', ['run', './cmd/seed'], { cwd: api, env: { ...process.env, ...demoEnv } });
}

const task = process.argv[2];
switch (task) {
  case 'gen':
    gen();
    break;
  case 'seed':
    seed();
    break;
  case 'test-api':
    run('go', ['test', './...'], { cwd: api });
    break;
  case 'dev': {
    run('pnpm', ['--filter', '@sushi-latte/api-client', 'gen'], { cwd: root });
    run('pnpm', ['--filter', '@sushi-latte/ui', 'tokens'], { cwd: root });
    if (!existsSync(join(root, 'data', 'sushi-latte.db'))) seed();
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
    if (!existsSync(join(root, 'data', 'sushi-latte.db'))) seed();
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
