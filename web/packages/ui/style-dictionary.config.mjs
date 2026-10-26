// Builds CSS variables and motion constants from docs/design/tokens/*.json (TOKENS.md §5).
// Generated files land in src/tokens/ and are never edited by hand.
import { copyFile, mkdir, readdir } from 'node:fs/promises';
import StyleDictionary from 'style-dictionary';

const tokens = (name) => `../../../docs/design/tokens/${name}.json`;
const fromFile = (name) => (token) => token.filePath.replaceAll('\\', '/').endsWith(`/${name}.json`);
const buildPath = 'src/tokens/';
const css = (destination, selector, file) => ({
  transformGroup: 'css',
  prefix: 'sl',
  buildPath,
  files: [{ destination, format: 'css/variables', filter: fromFile(file), options: { selector, outputReferences: true } }],
});

const builds = [
  // Primitives + light semantic roles on :root
  {
    source: [tokens('primitive'), tokens('semantic.light')],
    platforms: {
      primitive: css('primitive.css', ':root', 'primitive'),
      light: css('semantic.light.css', ':root', 'semantic.light'),
      motion: {
        transformGroup: 'js',
        buildPath,
        files: ['javascript/es6', 'typescript/es6-declarations'].map((format) => ({
          destination: format === 'javascript/es6' ? 'motion.js' : 'motion.d.ts',
          format,
          filter: (t) => ['duration', 'easing', 'spring'].includes(t.path[0]),
        })),
      },
    },
  },
  // Dark semantic roles under [data-theme="dark"]
  {
    include: [tokens('primitive')],
    source: [tokens('semantic.dark')],
    platforms: { dark: css('semantic.dark.css', '[data-theme="dark"]', 'semantic.dark') },
  },
  // One block per surface; values reference semantic variables so themes flow through.
  ...['customer', 'staff', 'admin', 'kitchen', 'display'].map((surface) => ({
    include: [tokens('primitive'), tokens('semantic.light')],
    source: [tokens(`surface.${surface}`)],
    platforms: { [surface]: css(`surface.${surface}.css`, `[data-surface="${surface}"]`, `surface.${surface}`) },
  })),
  // Component tokens reference surface aliases (radius.control…) resolved at runtime.
  {
    include: [tokens('primitive'), tokens('semantic.light'), tokens('surface.customer')],
    source: [tokens('component')],
    platforms: { component: css('component.css', ':root', 'component') },
  },
];

for (const config of builds) {
  const sd = new StyleDictionary({ ...config, log: { verbosity: 'silent', warnings: 'disabled' } });
  await sd.buildAllPlatforms();
}

// Brand assets: docs/brand is the source of truth; apps import the copies in src/brand.
const brandSrc = new URL('../../../docs/brand/', import.meta.url);
const brandOut = new URL('./src/brand/', import.meta.url);
await mkdir(brandOut, { recursive: true });
for (const file of (await readdir(brandSrc)).filter((f) => /\.(svg|png)$/.test(f))) {
  await copyFile(new URL(file, brandSrc), new URL(file, brandOut));
}
console.log('tokens and brand assets built');
