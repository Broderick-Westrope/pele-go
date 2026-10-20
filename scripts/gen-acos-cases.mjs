// Writes internal/jsmath/testdata/acos.json: Math.acos as V8 computes it, across each branch
// of fdlibm's acos (|x| < 2^-57, |x| < 0.5, x < -0.5, x > 0.5, |x| >= 1) and their edges.
// internal/jsmath replays it.
//
// Usage: node scripts/gen-acos-cases.mjs
import { writeFileSync } from 'node:fs';

let seed = 0x2545f491;
function random() {
  seed ^= seed << 13;
  seed ^= seed >>> 17;
  seed ^= seed << 5;
  return (seed >>> 0) / 0x100000000;
}

const args = [0, 1, -1, 0.5, -0.5, 1 - 2 ** -53, -1 + 2 ** -53, 2 ** -57, -(2 ** -57), 2 ** -58, 2 ** -56, 1e-300, 1.5, -2, 1e300];
for (const edge of [0.5, -0.5, 0.975, -0.975]) for (let k = -20; k <= 20; k++) args.push(edge + k * 2 ** -53);
for (let i = 0; i < 4000; i++) args.push(random() * 2 - 1);
for (let i = 0; i < 1000; i++) args.push((random() < 0.5 ? -1 : 1) * (1 - random() * 2 ** -Math.floor(random() * 50)));
for (let i = 0; i < 500; i++) args.push((random() - 0.5) * 2 ** -Math.floor(random() * 70));
const cases = args.map((x) => ({ fn: 'acos', args: [x], want: Math.acos(x) }));
writeFileSync(new URL('../internal/jsmath/testdata/acos.json', import.meta.url), JSON.stringify(cases) + '\n');
