// Writes internal/jsnum/testdata/fixed.json: Number.prototype.toFixed, toExponential and
// toPrecision as V8 answers them, around rounding ties, carries and the notation limits.
// internal/jsnum replays them.
//
// Usage: node scripts/gen-jsnum-fixed-cases.mjs
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

// Numbers travel as strings, so NaN, the infinities and -0 survive JSON.
const n = (v) => (Object.is(v, -0) ? '-0' : String(v));

let seed = 0x9e3779b9;
const random = () => {
  seed ^= seed << 13;
  seed ^= seed >>> 17;
  seed ^= seed << 5;
  return (seed >>> 0) / 4294967296;
};

const values = [
  0, -0, NaN, Infinity, -Infinity, 0.5, 1.5, 2.5, -2.5, 0.05, 0.15, 0.25, 0.35, 1.005, 1.045, 9.95, 9.995, 99.5, 999.9999,
  0.000001, 1e-7, 1.23e-7, 5e-324, Number.MIN_VALUE, Number.MAX_VALUE, 1e20, 1e21, 999999999999999999999, 123456789.123456789,
  0.1, 0.2, 0.3, 1 / 3, 2 / 3, 1234.5678, 1e-10, 4.35, 8.345, 1.25, 12, 100, 1e15, 2 ** 53, 2 ** 53 + 2, 0.0000095, 0.95,
];
for (let i = 0; i < 300; i++) {
  const scale = 10 ** Math.floor(random() * 30 - 12);
  values.push((random() - 0.3) * scale);
  values.push(Math.round(random() * 100000) / (10 ** Math.floor(random() * 6)));
}

const cases = [];
for (const v of values) {
  for (const f of [0, 1, 2, 3, 6, 12, 20]) {
    cases.push({ fn: 'toFixed', args: [n(v), f], want: v.toFixed(f) });
    cases.push({ fn: 'toExponential', args: [n(v), f], want: v.toExponential(f) });
  }
  for (const p of [1, 2, 3, 6, 12, 20, 21]) cases.push({ fn: 'toPrecision', args: [n(v), p], want: v.toPrecision(p) });
}

const out = join(dirname(fileURLToPath(import.meta.url)), '..', 'internal', 'jsnum', 'testdata', 'fixed.json');
writeFileSync(out, '[' + cases.map((c) => JSON.stringify(c)).join(',\n') + ']\n');
