// Writes internal/jsmath/testdata/pow.json: x ** y as V8 computes it, for the port of
// Math.pow. Each case is [x, y, x ** y] as the hex of the doubles' bits, so -0, NaN and the
// infinities survive JSON.
//
// Usage: node scripts/gen-pow-cases.mjs
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

let seed = 0x1d872b41;
function random() {
  seed ^= seed << 13;
  seed ^= seed >>> 17;
  seed ^= seed << 5;
  return (seed >>> 0) / 0x100000000;
}
const pick = (list) => list[Math.floor(random() * list.length)];

const view = new DataView(new ArrayBuffer(8));
function bits(x) {
  view.setFloat64(0, x);
  return view.getBigUint64(0).toString(16).padStart(16, '0');
}

const pow = (x, y) => x ** y;
const cases = [];
const add = (x, y) => cases.push([bits(x), bits(y), bits(pow(x, y))]);

const special = [0, -0, 1, -1, 0.5, -0.5, 2, -2, 3, 10, -10, 1e-310, -1e-310, 1e300, -1e300, Infinity, -Infinity, NaN, 1 + 2 ** -52, 1 - 2 ** -53];
for (const x of special) for (const y of special) add(x, y);
for (let n = -400; n <= 400; n++) add(10, n);
for (let n = -1100; n <= 1100; n += 7) add(2, n);
for (let i = 0; i < 3000; i++) {
  const x = (random() - 0.3) * 2 ** (Math.floor(random() * 80) - 40);
  const y = pick([
    () => Math.floor((random() - 0.5) * 60),
    () => Math.floor((random() - 0.5) * 2000) / 2,
    () => (random() - 0.5) * 2 ** (Math.floor(random() * 20) - 10),
    () => (random() - 0.5) * 2 ** (Math.floor(random() * 70) - 2),
  ])();
  add(x, y);
}
for (let i = 0; i < 500; i++) {
  add(1 + (random() - 0.5) * 2 ** -(Math.floor(random() * 30) + 20), (random() - 0.5) * 2 ** (Math.floor(random() * 40) + 25));
  add((random() - 0.5) * 2 ** (Math.floor(random() * 2000) - 1000), (random() - 0.5) * 8);
}

const file = new URL('../internal/jsmath/testdata/pow.json', import.meta.url);
mkdirSync(dirname(fileURLToPath(file)), { recursive: true });
writeFileSync(file, JSON.stringify(cases) + '\n');
