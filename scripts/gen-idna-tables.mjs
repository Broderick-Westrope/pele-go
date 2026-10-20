// Writes internal/util/idna_tables.go: the Unicode data of Node's domain to ASCII. Node's URL
// parser is ada, whose IDNA carries its own tables (IDNA mapping, NFC, Bidi_Class, joining
// types, viramas, marks), which are not the Unicode version V8 and ICU report. So the tables
// come from the ada source Node is built from, read through the lookups ada does.
//
// Usage: node scripts/gen-idna-tables.mjs [ada.cpp]
// Without a file it downloads deps/ada/ada.cpp of the running Node's tag.
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const adaURL = `https://raw.githubusercontent.com/nodejs/node/${process.version}/deps/ada/ada.cpp`;
let src;
if (process.argv[2]) {
  src = readFileSync(process.argv[2], 'utf8');
} else {
  const res = await fetch(adaURL);
  if (!res.ok) throw new Error(`${adaURL}: ${res.status}`);
  src = await res.text();
}

// The values of a C array initialiser, flattened, after the first match of name from `from`.
function array(name, from = 0) {
  const re = new RegExp(`\\b${name}(\\[[^\\]]*\\])+\\s*=\\s*\\{`, 'g');
  re.lastIndex = from;
  const m = re.exec(src);
  if (!m) throw new Error(`no array ${name}`);
  const body = src.slice(m.index + m[0].length, src.indexOf('};', m.index));
  return body.match(/0x[0-9a-fA-F]+|\d+|direction::\w+/g).map((v) => (v.startsWith('direction::') ? v.slice(11) : Number(v)));
}
const idnaVersion = src.match(/\/\/ IDNA\s+(\d+\.\d+\.\d+)/)[1];
const b36 = (n) => n.toString(36);

// Space-separated base-36 numbers, wrapped as Go string concatenation.
function packed(name, comment, nums) {
  const lines = [];
  let line = '';
  for (const n of nums) {
    const tok = b36(n);
    if (line.length + tok.length + 1 > 96) {
      lines.push(line + ' ');
      line = '';
    }
    line += (line ? ' ' : '') + tok;
  }
  lines.push(line);
  return `// ${comment}\nconst ${name} = ${lines.map((l) => JSON.stringify(l)).join(' +\n\t')}\n`;
}

// IDNA mapping: ada's ranges, each a start and a descriptor (status in the low byte; for
// mapped, the length and offset of its code points in `mappings`).
const table = array('table');
const mappings = array('mappings');
const mapping = [];
let prev = 0;
let lastMapped = 0;
for (let i = 0; i < table.length; i += 2) {
  const [start, desc] = [table[i], table[i + 1]];
  const status = desc & 0xff;
  if (status < 3) mapping.push(start - prev, status);
  else {
    const n = desc >>> 24;
    const at = (desc >>> 8) & 0xffff;
    mapping.push(start - prev, 3 + n);
    for (const m of mappings.slice(at, at + n)) {
      mapping.push(m - lastMapped);
      lastMapped = m;
    }
  }
  prev = start;
}

// NFC: canonical decompositions and combining classes per code point, composition pairs.
const decompIndex = array('decomposition_index');
const decompBlock = array('decomposition_block');
const decompData = array('decomposition_data');
const cccIndex = array('canonical_combining_class_index');
const cccBlock = array('canonical_combining_class_block');
const compIndex = array('composition_index');
const compBlock = array('composition_block');
const compData = array('composition_data');
const decomposition = [];
const ccc = [];
const composition = [];
let lastDecomp = 0;
let lastFirst = 0;
let run = null;
for (let c = 0; c < 0x110000; c++) {
  if (c < 0xac00 || c >= 0xac00 + 19 * 21 * 28) {
    const at = decompIndex[c >> 8] * 257 + (c % 256);
    const d0 = decompBlock[at];
    const len = (decompBlock[at + 1] >> 2) - (d0 >> 2);
    if (len > 0 && !(d0 & 1)) {
      decomposition.push(c - lastDecomp, len, ...decompData.slice(d0 >> 2, (d0 >> 2) + len));
      lastDecomp = c;
    }
  }
  const k = cccBlock[cccIndex[c >> 8] * 256 + (c % 256)];
  if (run && run.k === k && run.end === c - 1) run.end = c;
  else {
    if (run && run.k) ccc.push(run);
    run = { start: c, end: c, k };
  }
  const cat = compIndex[c >> 8] * 257 + (c % 256);
  for (let p = compBlock[cat]; p < compBlock[cat + 1]; p += 2) {
    composition.push(c - lastFirst, compData[p], compData[p + 1]);
    lastFirst = c;
  }
}
if (run.k) ccc.push(run);
const cccNums = [];
let end = -1;
for (const r of ccc) {
  cccNums.push(r.start - end - 1, r.end - r.start, r.k);
  end = r.end;
}

// Ranges as gap from the last range's end and length less one.
function rangeNums(ranges, extra = () => []) {
  const out = [];
  let last = -1;
  for (const r of ranges) {
    out.push(r[0] - last - 1, r[1] - r[0], ...extra(r));
    last = r[1];
  }
  return out;
}
function setRanges(points) {
  const out = [];
  for (const p of points) {
    const last = out[out.length - 1];
    if (last && last[1] === p - 1) last[1] = p;
    else out.push([p, p]);
  }
  return out;
}
const directions = ['NONE', 'BN', 'CS', 'ES', 'ON', 'EN', 'L', 'R', 'NSM', 'AL', 'AN', 'ET', 'WS', 'RLO', 'LRO', 'PDF', 'RLE', 'RLI', 'FSI', 'PDI', 'LRI', 'B', 'S', 'LRE'];
const dirTable = array('dir_table');
const dirRanges = [];
for (let i = 0; i < dirTable.length; i += 3) {
  const [start, final, d] = [dirTable[i], dirTable[i + 1], directions.indexOf(dirTable[i + 2])];
  if (d < 0) throw new Error(`direction ${dirTable[i + 2]}`);
  const last = dirRanges[dirRanges.length - 1];
  if (last && last[2] === d && last[1] === start - 1) last[1] = final;
  else dirRanges.push([start, final, d]);
}
const valid = src.indexOf('bool is_label_valid');

const go = `// Code generated by scripts/gen-idna-tables.mjs from ada ${process.versions.ada} in Node ${process.version} (IDNA ${idnaVersion}); DO NOT EDIT.

package util

// The tables are lists of base-36 numbers; idna.go reads them.

${packed('idnaMapping', 'IDNA mapping ranges: gap from the last start, status (0 ignored, 1 valid, 2 disallowed, or 3 plus the count of code points it maps to), and those code points, each less the last one mapped to.', mapping)}
${packed('nfcDecompositions', 'Canonical decompositions: gap from the last code point, count, code points.', decomposition)}
${packed('nfcCombiningClasses', 'Canonical combining classes other than 0: gap from the last range, length less one, class.', cccNums)}
${packed('nfcCompositions', 'Canonical compositions: gap from the last first code point, second, composite.', composition)}
${packed('bidiClasses', 'Bidi classes (ada\'s numbering: ' + directions.map((d, i) => `${i} ${d}`).join(', ') + '): gap from the last range, length less one, class.', rangeNums(dirRanges, (r) => [r[2]]))}
${packed('idnaMarks', 'Marks a label may not begin with: gap from the last range, length less one.', rangeNums(setRanges(array('combining', valid))))}
${packed('idnaViramas', 'Viramas: gap from the last range, length less one.', rangeNums(setRanges(array('virama', valid))))}
${packed('joiningR', 'Joining_Type R: gap from the last range, length less one.', rangeNums(setRanges(array('R', valid))))}
${packed('joiningL', 'Joining_Type L: gap from the last range, length less one.', rangeNums(setRanges(array('L', valid))))}
${packed('joiningD', 'Joining_Type D: gap from the last range, length less one.', rangeNums(setRanges(array('D', valid))))}`;

writeFileSync(fileURLToPath(new URL('../internal/util/idna_tables.go', import.meta.url)), go);
console.log(`mapping ${table.length / 2} ranges, ${decomposition.length} decomposition numbers, ${ccc.length} ccc ranges, ${composition.length / 3} compositions, ${dirRanges.length} bidi ranges`);
