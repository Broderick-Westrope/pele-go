// Runs upstream's Vitest suite with scripts/capture/capture-setup.ts and writes every
// public parse() and render() call it made to internal/oracle/testdata/captured/<type>.jsonl.gz,
// one file per detected diagram type, with a manifest that counts what could not be replayed.
//
// Usage: node scripts/capture-cases.mjs <pele checkout at 1.7.0, with npm ci run>
import { execFileSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const [, , peleDir] = process.argv;
if (!peleDir) throw new Error('usage: capture-cases.mjs <pele checkout>');

const here = fileURLToPath(new URL('.', import.meta.url));
const testdata = join(here, '../internal/oracle/testdata');
const setup = join(peleDir, 'tests/support/__capture-setup.ts');
const config = join(peleDir, 'vitest.capture.config.ts');
const dir = mkdtempSync(join(tmpdir(), 'pele-capture-'));

copyFileSync(join(here, 'capture/capture-setup.ts'), setup);
writeFileSync(
  config,
  `import base from './vitest.config.ts';
import { defineConfig, mergeConfig } from 'vitest/config';
export default mergeConfig(base, defineConfig({
  test: { setupFiles: ['./tests/support/__capture-setup.ts'], exclude: ['tests/website/**', 'node_modules/**'] },
}));
`
);
try {
  // Upstream tests may fail once calls are wrapped (a fixed clock, for one); only the calls matter.
  execFileSync('npx', ['vitest', 'run', '--config', config, '--reporter=dot'], {
    cwd: peleDir,
    env: { ...process.env, PELE_CAPTURE_DIR: dir },
    stdio: 'inherit',
  });
} catch {
  console.error('some upstream tests failed with capture on; continuing');
} finally {
  rmSync(setup);
  rmSync(config);
}

// Shard by detected diagram type, so each type's cases can be committed when it is ported.
const vm = await import('node:vm');
vm.runInThisContext(readFileSync(join(testdata, 'pele.global.js'), 'utf8'));
const typeOf = (src) => {
  try {
    return globalThis.pele.detectType(src) ?? 'none';
  } catch {
    return 'none';
  }
};

const seen = new Set();
const shards = {};
const manifest = { calls: 0, unique: 0, replayable: 0, unsupported: {}, byType: {} };
for (const file of readdirSync(dir).sort()) {
  for (const line of readFileSync(join(dir, file), 'utf8').split('\n')) {
    if (!line) continue;
    manifest.calls++;
    if (seen.has(line)) continue;
    seen.add(line);
    const record = JSON.parse(line);
    const reason = record.unsupported ?? record.parse?.unsupported ?? (record.outcome.startsWith('thrown:') ? record.outcome : undefined);
    if (reason) {
      manifest.unsupported[reason] = (manifest.unsupported[reason] ?? 0) + 1;
      continue;
    }
    const type = typeOf(record.src ?? record.parse.src);
    (shards[type] ??= []).push(line);
  }
}
manifest.unique = seen.size;
rmSync(dir, { recursive: true });
const out = join(testdata, 'captured');
mkdirSync(out, { recursive: true });
for (const [type, lines] of Object.entries(shards).sort()) {
  lines.sort();
  manifest.replayable += lines.length;
  manifest.byType[type] = lines.length;
  writeFileSync(join(out, `${type}.jsonl.gz`), gzipSync(lines.join('\n') + '\n', { level: 9 }));
}
// Calls that cannot be replayed are a gap in the parity gate. More of them than last time
// fails the capture instead of quietly narrowing the gate.
const total = (m) => Object.values(m.unsupported).reduce((a, b) => a + b, 0);
const manifestPath = join(out, 'manifest.json');
let baseline;
try {
  baseline = JSON.parse(readFileSync(manifestPath, 'utf8'));
} catch {
  baseline = undefined;
}
writeFileSync(manifestPath, JSON.stringify(manifest, null, 1) + '\n');
console.log(manifest);
if (baseline && total(manifest) > total(baseline)) {
  console.error(`unreplayable calls rose from ${total(baseline)} to ${total(manifest)}`);
  process.exit(1);
}
