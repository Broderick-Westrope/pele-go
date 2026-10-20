// Writes the differential fixtures for the shared modules (internal/util, internal/svg,
// internal/diagrams/common, internal/jsmath): calls of upstream's functions under V8 and what
// they returned. Each Go package's tests replay its file through internal/fixture.
//
// Usage: node scripts/gen-shared-cases.mjs <pele checkout at 1.7.0>
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const [, , peleDir] = process.argv;
if (!peleDir) throw new Error('usage: gen-shared-cases.mjs <pele checkout>');

// The modules import each other as .js; esbuild resolves them to the .ts sources.
const esbuild = createRequire(join(peleDir, 'package.json'))('esbuild');
const modules = {
  chars: 'src/util/chars.ts',
  unicode: 'src/util/unicode.ts',
  url: 'src/util/url.ts',
  reference: '@braintree/sanitize-url',
  builder: 'src/svg/builder.ts',
  theme: 'src/svg/theme.ts',
  title: 'src/svg/title.ts',
  cluster: 'src/svg/cluster.ts',
  edges: 'src/svg/edges.ts',
  shapes: 'src/svg/shapes.ts',
  measurer: 'src/text/measurer.ts',
  label: 'src/text/label.ts',
  tokens: 'src/diagrams/common/tokens.ts',
  accDescr: 'src/diagrams/common/accDescr.ts',
  caseless: 'src/diagrams/common/caseless.ts',
  scan: 'src/diagrams/common/scan.ts',
  outline: 'src/diagrams/common/outline.ts',
  generics: 'src/diagrams/common/generics.ts',
  icon: 'src/diagrams/common/icon.ts',
  fit: 'src/diagrams/common/fit.ts',
  fitText: 'src/diagrams/common/fit-text.ts',
  fitWidth: 'src/diagrams/common/fit-width.ts',
  literalLabel: 'src/diagrams/common/literal-label.ts',
};
const entry = Object.entries(modules)
  .map(([name, path]) => `export * as ${name} from ${JSON.stringify(path.startsWith('src/') ? './' + path : path)};`)
  .join('\n');
const out = join(mkdtempSync(join(tmpdir(), 'pele-cases-')), 'bundle.mjs');
await esbuild.build({
  stdin: { contents: entry, resolveDir: peleDir, loader: 'ts' },
  bundle: true,
  format: 'esm',
  platform: 'node',
  outfile: out,
  logLevel: 'error',
});
const m = await import(pathToFileURL(out).href);

// A seeded generator, so the fixtures only change when upstream does.
let seed = 0x2545f491;
function random() {
  seed ^= seed << 13;
  seed ^= seed >>> 17;
  seed ^= seed << 5;
  return (seed >>> 0) / 0x100000000;
}
const pick = (list) => list[Math.floor(random() * list.length)];
// Strings joined from pieces, for fuzz-like inputs.
function compose(pieces, max) {
  let s = '';
  const n = 1 + Math.floor(random() * max);
  for (let i = 0; i < n; i++) s += pick(pieces);
  return s;
}

function write(path, cases) {
  const file = new URL('../' + path, import.meta.url);
  mkdirSync(dirname(fileURLToPath(file)), { recursive: true });
  writeFileSync(file, JSON.stringify(cases) + '\n');
}

// What a call returned, as JSON can carry it: undefined becomes null, and a thrown error its message.
function call(fn, name, args) {
  let want;
  try {
    want = fn(...args);
  } catch (e) {
    return { fn: name, args, throws: String(e && e.message) };
  }
  return { fn: name, args, want: want === undefined ? null : want };
}

const sections = {};
const section = (name, build) => (sections[name] = build);

section('util', () => {
  const cases = [];
  const letters = [];
  for (let c = 0; c <= 0x10000; c++) {
    if (!m.unicode.isUnicodeLetter(c)) continue;
    const last = letters[letters.length - 1];
    if (last && last[1] === c - 1) last[1] = c;
    else letters.push([c, c]);
  }
  cases.push({ fn: 'letters', args: [], want: letters });
  // Every code unit any of the tests holds for, with a bit per test.
  const tests = [m.chars.isSpace, m.chars.isBlank, m.chars.isLineEnd, m.chars.isDigit, m.chars.isLetter, m.chars.isHex, m.chars.isWord];
  const chars = [];
  for (let c = 0; c <= 0x10000; c++) {
    const mask = tests.reduce((acc, f, k) => acc | (f(c) ? 1 << k : 0), 0);
    if (mask) chars.push([c, mask]);
  }
  chars.push([-1, tests.reduce((acc, f, k) => acc | (f(NaN) ? 1 << k : 0), 0)]);
  cases.push({ fn: 'chars', args: [], want: chars });
  return cases;
});

section('jsmath', () => {
  const cases = [];
  const args = [0, -0, 1, -1, Math.PI, Math.PI / 2, -Math.PI / 4, 1e-9, 1e300, -1e22, 2 ** 19 * Math.PI];
  for (let i = 0; i < 28; i++) args.push((Math.PI * i) / 14 - Math.PI / 2);
  for (let i = 0; i < 2500; i++) args.push((random() - 0.5) * 2 ** (Math.floor(random() * 90) - 30));
  for (let i = 0; i < 300; i++) args.push((random() - 0.5) * 2 ** (Math.floor(random() * 2000) - 1000));
  for (let i = 1; i < 100; i++) args.push((i * Math.PI) / 4);
  for (const x of args) cases.push(call(Math.sin, 'sin', [x]), call(Math.cos, 'cos', [x]));
  for (let i = 0; i < 1000; i++) {
    const n = Math.floor(random() * 4);
    const xs = [];
    for (let k = 0; k < n; k++) xs.push((random() - 0.5) * 2 ** (Math.floor(random() * 60) - 30));
    cases.push(call(Math.hypot, 'hypot', xs));
  }
  cases.push(call(Math.hypot, 'hypot', [3, 4]), call(Math.hypot, 'hypot', [1e200, 1e200]), call(Math.hypot, 'hypot', [0, -0]));
  return cases;
});

for (const [name, build] of Object.entries(sections)) {
  const target = { util: 'internal/util', svg: 'internal/svg', common: 'internal/diagrams/common', jsmath: 'internal/jsmath' }[name];
  write(`${target}/testdata/cases.json`, build());
}
