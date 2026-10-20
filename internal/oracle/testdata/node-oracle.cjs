// Reads one JSON request per line from stdin and writes one JSON response per line: what
// pele.render returns, or the PeleError it throws. Strings travel as arrays of UTF-16 code
// units, so lone surrogates survive JSON on both sides.
const { readFileSync } = require('node:fs');
const vm = require('node:vm');
const readline = require('node:readline');

vm.runInThisContext(readFileSync(process.argv[2], 'utf8'));
const pele = globalThis.pele;

const units = (s) => Array.from({ length: s.length }, (_, i) => s.charCodeAt(i));
const text = (u) => {
  let s = '';
  for (let i = 0; i < u.length; i += 8192) s += String.fromCharCode(...u.slice(i, i + 8192));
  return s;
};

// Mirrors lengthMeasurer in the Go tests.
const measurers = {
  length: { width: (t, size, style) => t.length * size * 0.55 + style * 0.25 },
};

const rl = readline.createInterface({ input: process.stdin });
rl.on('line', (line) => {
  const req = JSON.parse(line);
  const opts = { ...req.options };
  if (req.now !== undefined) opts.now = req.now;
  if (req.measurer) opts.measurer = measurers[req.measurer];
  let out;
  try {
    const r = pele.render(text(req.src), opts);
    out = {
      svg: units(r.svg),
      width: r.width,
      height: r.height,
      type: r.type,
      mermaidType: r.mermaidType,
      links: r.links.map((l) => ({ id: units(l.id), href: units(l.href), internal: l.internal })),
    };
  } catch (e) {
    if (e && e.name === 'PeleError') {
      out = {
        error: {
          code: e.code,
          message: units(e.message),
          type: e.type,
          mermaidType: e.mermaidType,
          line: e.line,
          column: e.column,
          snippet: units(e.snippet),
        },
      };
    } else {
      out = { thrown: String(e && e.stack) };
    }
  }
  process.stdout.write(JSON.stringify(out) + '\n');
});
