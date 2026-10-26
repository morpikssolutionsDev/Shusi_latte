// Copies favicon and PWA icons from docs/brand (the source of truth) into an app's public/.
//   node ../copy-brand.mjs
import { copyFile, mkdir } from 'node:fs/promises';

const brand = new URL('../../docs/brand/', import.meta.url);
const pub = new URL('public/', `file://${process.cwd().replaceAll('\\', '/')}/`);
await mkdir(pub, { recursive: true });
const files = {
  'favicon.svg': 'sushi-latte-app-icon.svg',
  'icon-192.png': 'sushi-latte-app-icon-192.png',
  'icon-512.png': 'sushi-latte-app-icon-512.png',
};
for (const [to, from] of Object.entries(files)) {
  await copyFile(new URL(from, brand), new URL(to, pub));
}
