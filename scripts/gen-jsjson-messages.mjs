// Writes internal/jsjson/testdata/messages.json: the message V8's JSON.parse throws for each of
// a set of texts, or null where it parses them. internal/jsjson replays them.
//
// Usage: node scripts/gen-jsjson-messages.mjs
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const HAND = [
  '', ' ', '{', '[', '}', ']', '{}', '[]', '{} x', '{}}', '{  }  }', 'NaN', 'Infinity', 'undefined', '[object Object]',
  'nul', 'tru', 'fals', 'truex', 't', '"', '"abc', '"\\', '"\\u12', '-', '-a', '01', '-01', '1.', '1e', '1e+', '0x',
  '{"a":1,}', '{"a":}', '{a:1}', '{"a" 1}', '{"a":1 "b":2}', '["a" "b"]', '[1,]', '{"a":01}', '{"a":-}',
  '{"a":1.}', '{"a":1e}', '{"a":"\n"}', '{"a":"\\x"}', '{"a":"\\u12g4"}', '{"a":"abc', '{"a":tru}', '{"a":nul}',
  '{"a":t1}', '{"a":t""}', '{"a":t }', '{"a":"\\中"}', '{"a":"\\é"}', '{"a":-a}', '{"a":0-}', '{"a":-01}',
  '{"a":1.5x}', '{"a":1E5}', '{"a":"\u0001"}', '{"a":\u2028}', '{"a"\n\n  :x}', '{\r\n"a":x}', '{\r\r\n\n"a":x}',
  '{"a":\ud83d\ude00}', '{"\ud800":x}', '{"a":"\ud800"x}', '{"a":12345678901x}', '{"a":[}', '{"a":{"b":1},}',
  '{"a":\'x\'}', '{"a":[1 2]}', '{"position 5":x}', '{"line 3 column 4":x}', '{"unexpected end":x}',
  '{"a":"\\', '{"a":1}\n\n x', '{\n  "a": [1, 2,\n  3,],\n}', '{"a":-0}', '{"a":-0.0e-0}', '{"a":1e400}',
];

const ALPHABET = [...'{}[]:,"\\ \n\r\t0123456789-+.eEtrufalsnx/b', '\u0001', '\u001f', 'é', '中', '\u2028', '\ud83d', '\ude00'];
const SEEDS = [
  '{"a":1}',
  '{"name":"Ada","tags":["x","y"],"n":-1.5e3}',
  '{"a":{"b":{"c":[true,false,null]}},"d":"\\u00e9\\n"}',
  '{\n  "id": 7,\n  "list": [1, 2, {"k": "v"}],\n  "ok": true\n}',
  '{"aaaaaaaaaaaaaaaaaaaa":"bbbbbbbbbbbbbbbbbbbb","cccccccccc":[0.25,10,-0]}',
];

let seed = 0x2545f491;
const random = () => {
  seed ^= seed << 13;
  seed ^= seed >>> 17;
  seed ^= seed << 5;
  return (seed >>> 0) / 4294967296;
};
const int = (n) => Math.floor(random() * n);

function mutate(text) {
  let s = text;
  for (let k = 1 + int(3); k > 0; k--) {
    const at = int(s.length + 1);
    const ch = ALPHABET[int(ALPHABET.length)];
    const op = int(3);
    if (op === 0) s = s.slice(0, at) + ch + s.slice(at);
    else if (op === 1) s = s.slice(0, at) + s.slice(at + 1);
    else s = s.slice(0, at) + ch + s.slice(at + 1);
  }
  return s;
}

const inputs = [...HAND];
for (const text of SEEDS) for (let i = 0; i < 600; i++) inputs.push(mutate(text));

const cases = inputs.map((text) => {
  try {
    JSON.parse(text);
    return [text, null];
  } catch (error) {
    return [text, error.message];
  }
});

const out = join(dirname(fileURLToPath(import.meta.url)), '..', 'internal', 'jsjson', 'testdata', 'messages.json');
writeFileSync(out, '[\n' + cases.map((c) => JSON.stringify(c)).join(',\n') + '\n]\n');
console.log(`${cases.length} cases written to ${out}`);
