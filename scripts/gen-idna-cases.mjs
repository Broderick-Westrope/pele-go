// Writes internal/util/testdata/idna.json.gz: what Node's URL parser (ada) makes of
// http://<host>/ for many hosts, as the differential fixture of the port's IDNA.
//
// - hosts: [host, href or null where new URL() throws], from Unicode's IdnaTestV2 files,
//   every combination of up to three of a set of bidi and joiner representatives, the
//   decomposed form of every precomposed character, xn-- labels valid and broken, and
//   seeded random strings.
// - sweeps: one line per code point (the href, or "!"), hashed, for a pattern holding the
//   code point. "all" sweeps every scalar value; "live" only those that "{c}" or "a{c}"
//   accept. {c} is the code point and {p} its Punycode.
//
// Usage: node scripts/gen-idna-cases.mjs [IdnaTestV2.txt ...]
// Without files it downloads IdnaTestV2.txt for IDNA 15.0.0, 15.1.0 and 16.0.0.
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

let seed = 0x1d4a6b35;
function random() {
  seed ^= seed << 13;
  seed ^= seed >>> 17;
  seed ^= seed << 5;
  return (seed >>> 0) / 0x100000000;
}
const pick = (list) => list[Math.floor(random() * list.length)];

function href(host) {
  try {
    return new URL('http://' + host + '/').href;
  } catch {
    return null;
  }
}

// Punycode (RFC 3492) of any code points, without IDNA's checks, for broken xn-- labels.
function punycode(cps) {
  const adapt = (delta, n, first) => {
    delta = first ? Math.floor(delta / 700) : delta >> 1;
    delta += Math.floor(delta / n);
    let k = 0;
    for (; delta > 455; k += 36) delta = Math.floor(delta / 35);
    return k + Math.floor((36 * delta) / (delta + 38));
  };
  const digit = (d) => String.fromCharCode(d < 26 ? 97 + d : 22 + d);
  let out = cps.filter((c) => c < 0x80).map((c) => String.fromCharCode(c)).join('');
  const b = out.length;
  let h = b;
  if (b > 0) out += '-';
  let n = 128;
  let delta = 0;
  let bias = 72;
  while (h < cps.length) {
    const m = Math.min(...cps.filter((c) => c >= n));
    delta += (m - n) * (h + 1);
    n = m;
    for (const c of cps) {
      if (c < n) delta++;
      if (c === n) {
        let q = delta;
        for (let k = 36; ; k += 36) {
          const t = k <= bias ? 1 : k >= bias + 26 ? 26 : k - bias;
          if (q < t) break;
          out += digit(t + ((q - t) % (36 - t)));
          q = Math.floor((q - t) / (36 - t));
        }
        out += digit(q);
        bias = adapt(delta, h + 1, h === b);
        delta = 0;
        h++;
      }
    }
    delta++;
    n++;
  }
  return out;
}
const cpsOf = (s) => [...s].map((ch) => ch.codePointAt(0));

// Characters that change what new URL() parses rather than the host.
const delimiter = /[:/?#\\@[]/;
const hosts = new Map();
function add(host) {
  if (delimiter.test(host) || hosts.has(host) || !host.isWellFormed()) return;
  hosts.set(host, href(host));
}

// IdnaTestV2: the source, toUnicode and toASCII(N) columns.
const files = process.argv.slice(2);
const texts = [];
if (files.length) {
  for (const f of files) texts.push(readFileSync(f, 'utf8'));
} else {
  for (const v of ['15.0.0', '15.1.0', '16.0.0']) {
    const res = await fetch(`https://www.unicode.org/Public/idna/${v}/IdnaTestV2.txt`);
    if (!res.ok) throw new Error(`IdnaTestV2 ${v}: ${res.status}`);
    texts.push(await res.text());
  }
}
const unescape = (s) =>
  s.replace(/\\u([0-9A-Fa-f]{4})|\\x\{([0-9A-Fa-f]+)\}/g, (_, a, b) => String.fromCodePoint(parseInt(a ?? b, 16)));
for (const text of texts) {
  for (const line of text.split('\n')) {
    const body = line.replace(/#.*/, '').trim();
    if (!body) continue;
    const cols = body.split(';').map((c) => unescape(c.trim()));
    const source = cols[0];
    add(source);
    if (cols[1]) add(cols[1]);
    if (cols[3]) add(cols[3]);
    add(source.toUpperCase());
    add('a.' + source);
  }
}

// Bidi classes, joining types, viramas, marks, mapped and ignored characters, separators.
const reps = [
  'a', 'Z', '1', '-', '+', ',', '$', '!', 'ß', 'ǅ', '\u03a3',
  '\u05d0', '\u05d1', '\u0627', '\u0628', '\u0644', '\u0660', '\u06f1', '\u0710', '\ua872',
  '\u0301', '\u0591', '\u094d', '\u0915', '\u200c', '\u200d', '\u00ad', '\u2028', '\u3002', '.',
  '\ufe0f', '\u0b47', '\u0b3e',
];
for (const x of reps) {
  add(x);
  for (const y of reps) {
    add(x + y);
    for (const z of reps) add(x + y + z);
  }
}

// Every precomposed character decomposed, alone and after a letter, and its marks reordered.
for (let c = 0x80; c <= 0x10ffff; c++) {
  if (c >= 0xd800 && c <= 0xdfff) continue;
  const ch = String.fromCodePoint(c);
  const nfd = ch.normalize('NFD');
  if (nfd === ch) continue;
  add(nfd);
  add('a' + nfd);
  const parts = [...nfd];
  if (parts.length > 2) add(parts[0] + parts.slice(1).reverse().join(''));
  if (parts.length > 1) add(parts[0] + '\u0323' + parts.slice(1).join(''));
}
for (const s of ['\u1100\u1161', '\u1100\u1161\u11a8', '\uac00\u11a8', '\uac00\u11a7', '\u1100\u1175\u11c2', '\u1112\u1175\u11c3', 'a\u0327\u0301\u0323', 'a\u0301\u0327', '\u0e40\u0e48\u0e01', 'e\u0302\u0301', '\u0928\u093c', '\u0915\u093c']) {
  add(s);
}

// xn-- labels: Node's own, then broken ones.
const pool = [
  ...'abcdefghijklmnopqrstuvwxyz0123456789-',
  ...'äöüéèçñßøåæœ', ...'абвгдежзийклмнопрстуфхцчшщъыьэюя', ...'αβγδεζηθικλμνξοπρστυφχψω',
  ...'אבגדהוזחטיכלמנסעפצקרשת', ...'ابتثجحخدذرزسشصضطظعغفقكلمنهوي', ...'٠١٢٣٤٥٦٧٨٩', ...'۰۱۲۳',
  ...'中文日本語漢字한국어조선말ไทยहिन्दीதமிழ்', '\u0301', '\u0308', '\u094d', '\u200c', '\u200d', '\u00ad',
  ...'ＡＢＣａｂｃ０１２', ...'ǅǈⅨ㍱ﬁ', 'ẞ', 'İ', 'Ω', '\u{1f600}', '\u{1d400}', '\u{10400}', '\u{20000}', '\u{e0100}',
  '\ufffd', '\u3002', '\uff0e', '%', '<', '^', '|', ' ', '\u0000', '\u007f', '\u0080', '\u00a0', '\ufeff',
];
const randomString = (max) => {
  let s = '';
  const n = 1 + Math.floor(random() * max);
  for (let i = 0; i < n; i++) s += pick(pool);
  return s;
};
const randomCodePoint = () => {
  for (;;) {
    const r = random();
    const c = r < 0.5 ? Math.floor(random() * 0x800) : r < 0.85 ? Math.floor(random() * 0x10000) : Math.floor(random() * 0x110000);
    if (c < 0xd800 || c > 0xdfff) return c;
  }
};
for (let i = 0; i < 3000; i++) {
  const s = randomString(8);
  const encoded = href(s);
  if (encoded) {
    const host = encoded.slice(7, -1);
    add(host);
    add(host.toUpperCase());
    for (const label of host.split('.')) {
      if (!label.startsWith('xn--')) continue;
      add(label);
      add(label.slice(0, -1));
      add(label + pick([...'a9z-']));
      add(label.slice(0, 4) + label.slice(5));
      add('Xn--' + label.slice(4));
      add(label.replace(/-[^-]*$/, ''));
      const k = 4 + Math.floor(random() * (label.length - 4));
      add(label.slice(0, k) + pick([...'aZ0-_~\u00e9']) + label.slice(k + 1));
    }
  }
  const cps = cpsOf(randomString(5));
  add('xn--' + punycode(cps));
  add('xn--' + punycode(cpsOf(String.fromCodePoint(...cps).toUpperCase())));
  add('xn--' + punycode(cpsOf(String.fromCodePoint(...cps).normalize('NFD'))));
  add('xn--' + punycode([randomCodePoint(), randomCodePoint()]));
}
for (const s of [
  'xn--', 'xn---', 'xn--a', 'xn--a-', 'xn--ab-', 'xn--9', 'xn--zz', 'xn--99999999', 'xn--zzzzzzzzzzzz', 'xn--5a',
  'xn--a.b', 'a.xn--', 'xn--ls8h', 'xn--LS8H', 'xn--zca', 'xn--Zca', 'xn--ss-', 'xn--abc-', 'xn--0', 'xn--ab',
  'xn-ab', 'xn-', 'XN--ZCA', 'xn--fiq228c', 'xn--wgv71a', 'xn--zfr164b', 'xn--a-ecp.ru', 'xn--d1abbgf6aiiy.xn--p1ai',
  'xn--n3h', 'xn--.a', '.xn--a-', 'xn--ü', 'xn--a\u00e4', 'ｘｎ--zca', 'xn\uff0d-zca', 'xn--zca\u3002', 'xn--8ca67lbac',
  'xn--4dbrk0ce', 'xn--a-ruu', 'xn--1ug', 'xn--0ug', 'xn--2ib', 'xn--ab-fsf', 'xn--l8h', 'xn--tda', 'xn--bss',
  'xn--zzzzzzz', 'xn--mgbh0fb', 'xn--mgbh0fb1', 'xn--ngbrx', 'xn--zzzzzzzzzzzzzzzzzzzzzz-', 'xn--a-zzzzzzzz',
  'xn--x', 'xn--7', 'xn--ba', 'xn--b-', 'xn--de-jg4avhby1noc0d', 'xn--hxajbheg2az3al', 'xn--mnchen-3ya',
]) {
  add(s);
  add(s + '.com');
  add('a.' + s);
}

// Random strings of the pool, of random code points, joined into labels, and percent-encoded.
for (let i = 0; i < 6000; i++) {
  let host = randomString(10);
  if (random() < 0.3) host += '.' + randomString(6);
  if (random() < 0.2) host = pick(['', 'xn--', 'www.', 'a.']) + host;
  add(host);
  let s = '';
  const n = 1 + Math.floor(random() * 6);
  for (let k = 0; k < n; k++) s += random() < 0.3 ? pick(pool) : String.fromCodePoint(randomCodePoint());
  add(s);
  if (i % 6 === 0) add(encodeURIComponent(host));
}
for (const s of ['', '.', '..', 'a..b', 'a.', '\u3002', '%2e', 'a%2Eb', '%41', '%e4%b8%ad', '%ef%bb%bf', '%ef%bb%bfa', '%ff', '%c3', '%c0%af', '%ed%a0%80',
  '%f4%90%80%80', 'a%00', '%20', 'A\u00adB', '\u00ad', '\ufe0f', '1.2.3.4', '0x7f.1', '1.2.3.\uff14', '\uff11\uff12\uff17.0.0.1', '0.0.0.\u0661',
  'xn--1.2.3.4', 'a.1', '\u05d0.1', '1.\u05d0', '\u05d0\u05d1.com', 'com.\u05d0a', 'a\u05d0', '\u05d0a', 'a\u05d01', '\u05d01\u0661']) {
  add(s);
}

// The sweeps.
const all = [];
for (let c = 0; c <= 0x10ffff; c++) if (c < 0xd800 || c > 0xdfff) all.push(c);
function sweep(name, pattern, domain) {
  const hash = createHash('sha256');
  for (const c of domain) {
    const ch = String.fromCodePoint(c);
    const host = pattern.replaceAll('{c}', ch).replaceAll('{p}', () => punycode([c]));
    hash.update((href(host) ?? '!') + '\n');
  }
  return { name, pattern, domain: domain === all ? 'all' : 'live', count: domain.length, sha256: hash.digest('hex') };
}
const live = all.filter((c) => href(String.fromCodePoint(c)) !== null || href('a' + String.fromCodePoint(c)) !== null);
const sweeps = [
  sweep('alone', '{c}', all),
  sweep('after a letter', 'a{c}', all),
  sweep('as Punycode', 'xn--{p}', all),
  sweep('as Punycode after a letter', 'xn--a{p}', live),
  sweep('inside RTL', '\u05d0{c}\u05d0', live),
  sweep('ending RTL', '\u05d0{c}', live),
  sweep('inside LTR of an RTL label', 'a{c}\u05d0', live),
  sweep('with Arabic-Indic digits', '\u0660{c}\u05d0', live),
  sweep('with European digits', '\u05d01{c}\u05d0', live),
  sweep('before ZWNJ', 'x{c}\u200c\u0628', live),
  sweep('before ZWJ', 'x{c}\u200d', live),
  sweep('after ZWNJ', '\u0628\u200c{c}', live),
  sweep('before an acute', '{c}\u0301', live),
  sweep('after a cedilla', 'a\u0327{c}', live),
];

const out = { node: process.version, ada: process.versions.ada, hosts: [...hosts], sweeps };
writeFileSync(fileURLToPath(new URL('../internal/util/testdata/idna.json.gz', import.meta.url)), gzipSync(JSON.stringify(out) + '\n', { level: 9 }));
console.log(`${hosts.size} hosts, ${sweeps.length} sweeps (${live.length} live code points)`);
