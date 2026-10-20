// Writes internal/jsdate/testdata/cases.json: JS Date calls as V8 answers them in several time
// zones, around their transitions, at the ends of the time range, and with fields that
// overflow. internal/jsdate replays them.
//
// Usage: node scripts/gen-jsdate-cases.mjs
import { execFileSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const ZONES = ['UTC', 'Australia/Sydney', 'America/New_York', 'Europe/London', 'Asia/Kolkata', 'Australia/Lord_Howe'];

// Numbers travel as strings, so NaN, the infinities and -0 survive JSON.
const n = (v) => (Object.is(v, -0) ? '-0' : String(v));

function casesIn(zone) {
  let seed = 0x2545f491;
  const random = () => {
    seed ^= seed << 13;
    seed ^= seed >>> 17;
    seed ^= seed << 5;
    return (seed >>> 0) / 4294967296;
  };
  const int = (lo, hi) => lo + Math.floor(random() * (hi - lo + 1));
  const pick = (list) => list[Math.floor(random() * list.length)];
  const cases = [];
  const fields = (t) => {
    const d = new Date(t);
    return [d.getFullYear(), d.getMonth(), d.getDate(), d.getDay(), d.getHours(), d.getMinutes(), d.getSeconds(),
      d.getMilliseconds(), d.getUTCSeconds(), d.getTimezoneOffset(), d.getTime()];
  };
  const get = (t) => cases.push({ fn: 'fields', args: [zone, n(t)], want: fields(t) });
  const local = (...a) => cases.push({ fn: 'local', args: [zone, ...a.map(n)], want: new Date(...a).getTime() });
  const set = (method, t, ...a) => {
    const d = new Date(t);
    d[method === 'setMonthDate' ? 'setMonth' : method](...a);
    cases.push({ fn: method, args: [zone, n(t), ...a.map(n)], want: d.getTime() });
  };
  const parse = (s) => cases.push({ fn: 'parse', args: [zone, s], want: new Date(s).getTime() });

  // Transitions between 1880 and 2045, to the millisecond.
  const transitions = [];
  const offset = (t) => new Date(t).getTimezoneOffset();
  for (let t = Date.UTC(1880, 0, 1); t < Date.UTC(2045, 0, 1); t += 86400000) {
    if (offset(t) === offset(t + 86400000)) continue;
    let lo = t;
    let hi = t + 86400000;
    while (hi - lo > 1) {
      const mid = Math.floor((lo + hi) / 2);
      if (offset(mid) === offset(lo)) lo = mid;
      else hi = mid;
    }
    transitions.push(hi);
  }
  const chosen = transitions.length > 40 ? transitions.filter((_, i) => i < 12 || i % 6 === 0 || i > transitions.length - 8) : transitions;
  for (const T of chosen) {
    for (const dt of [-86400000, -3600000, -1800000, -1, 0, 1, 1800000, 3600000, 86400000]) get(T + dt);
    // Wall times around the one where the clocks change, before and after it.
    for (const wall of [T - offset(T - 1) * 60000, T - offset(T) * 60000]) {
      for (const dm of [-61, -60, -31, -30, -1, 0, 1, 29, 30, 31, 59, 60, 61, 90, 121]) {
        const d = new Date(wall + dm * 60000);
        local(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), d.getUTCHours(), d.getUTCMinutes(), d.getUTCSeconds(), d.getUTCMilliseconds());
      }
    }
    const start = T - 2 * 86400000 + int(0, 86400000);
    set('setDate', start, new Date(start).getDate() + int(1, 4));
    set('setHours', T - 86400000, 0, 0, 0, 0);
    set('setHours', T + 86400000, 0, 0, 0, 0);
    set('setHours', T - 86400000, 23, 59, 59, 999);
    set('setHours', T - 3600000, int(0, 4), int(0, 59), 0, 0);
    set('setMonth', T - 31 * 86400000, new Date(T - 31 * 86400000).getMonth() + 1);
    set('setMonthDate', T - 3 * 86400000, 0, 1);
    set('setFullYear', T + 365 * 86400000, new Date(T).getFullYear());
  }

  // Ends of the range and local mean time.
  for (const t of [0, -0, 1, -1, 8.64e15, -8.64e15, 8.64e15 - 1, -8.64e15 + 1, 8.64e15 + 1, NaN, Infinity, 1.5, -1.5,
    -5364662400000, -1640014855492000, 253402300799999, -62198755200000]) {
    get(t);
    cases.push({ fn: 'new', args: [zone, n(t)], want: new Date(t).getTime() });
  }
  for (const y of [275760, -271821, 275759, -271820]) {
    for (const [mo, d] of [[8, 13], [8, 12], [3, 20], [3, 19], [0, 1], [11, 31]]) {
      local(y, mo, d, 0, 0, 0, 0);
      local(y, mo, d, 23, 59, 59, 999);
      local(y, mo, d, 12, 0, 0, 0);
    }
  }
  for (const y of [0, 49, 50, 99, 100, -1, 99.9, -0.5, 1800, 1850, 1900, 1895, 1915, 1941, 1945, 2038, 2100, 3000, 1e6, 1e6 + 1, -1e6, NaN, Infinity]) {
    local(y, 0, 1, 0, 0, 0, 0);
    local(y, 6, 15, 12, 30, 15, 500);
  }

  // Random fields, many of them overflowing.
  const big = () => pick([0, 1, -1, 12, 13, -13, 31, 32, 60, 1e3, 1e7, 1e7 + 1, -1e7, 1e9, 1e20, -1e20, 1.5, -1.5, NaN, Infinity]);
  for (let i = 0; i < 500; i++) {
    const year = random() < 0.8 ? int(1850, 2120) : pick([int(-300000, 300000), int(0, 99), big()]);
    const a = [year, int(-30, 30), int(-70, 70), int(-50, 50), int(-200, 200), int(-200, 200), int(-5000, 5000)];
    if (random() < 0.2) a[int(1, 6)] = big();
    if (random() < 0.5) a.splice(3, 4, int(0, 23), int(0, 59), int(0, 59), int(0, 999));
    local(...a);
    cases.push({ fn: 'utc', args: [zone, ...a.map(n)], want: Date.UTC(...a) });
  }
  const times = () =>
    random() < 0.7 ? int(Date.UTC(1890, 0, 1), Date.UTC(2110, 0, 1)) : pick([int(-8.64e15, 8.64e15), 8.64e15 - int(0, 1e10), -8.64e15 + int(0, 1e10), NaN]);
  for (let i = 0; i < 400; i++) {
    const t = times();
    get(t);
    const small = () => (random() < 0.9 ? int(-40, 400) : big());
    set('setDate', t, small());
    set('setMonth', t, random() < 0.9 ? int(-30, 30) : big());
    set('setMonthDate', t, int(-13, 13), small());
    set('setFullYear', t, random() < 0.9 ? int(1800, 2200) : pick([int(-300000, 300000), big()]));
    set('setHours', t, int(-30, 30), int(-70, 70), int(-70, 70), int(-2000, 2000));
    set('setTime', t, random() < 0.9 ? t + int(-1e9, 1e9) : big());
  }

  // Strings, as a gantt task's start can be anything the format did not match.
  const fixed = [
    '', ' ', '2024', '2024-01', '2024-01-05', '2024-1-5', '2024-01-05T10:00', '2024-01-05T10:00:00', '2024-01-05T10:00:00.123',
    '2024-01-05T10:00:00Z', '2024-01-05T10:00:00+05:30', '2024-01-05T10:00:00-0800', '2024-01-05T24:00', '2024-01-05T24:01',
    '2024-01-05T10:00:00.1234567891', '+002024-01-05', '-002024-01-05', '-000000-01-01', '+275760-09-13', '+275760-09-14',
    '-271821-04-20', '-271821-04-19T23:59', '2024-13-01', '2024-02-30', '2024-02-31T00:00', '20240105', '2024/01/05',
    '01/05/2024', '1/5/2024 10:00 PM', '1/5/2024 12:00 AM', '1/5/2024 13:00 PM', 'Jan 5 2024', 'January 5, 2024', '5 Jan 2024',
    '2024 Jan 5', 'Fri Jan 05 2024 10:00:00 GMT+1100 (Australian Eastern Daylight Time)', 'Fri, 05 Jan 2024 10:00:00 GMT',
    'Jan 5 2024 10:00 EST', 'Jan 5 2024 10:00 PDT', 'Jan 5 2024 10:00 UTC+5', 'Jan 5 2024 10:00 GMT-0800',
    'Jan 5 2024 10:00 GMT+05:30', 'Jan 5 2024 10:00 Z', 'Jan 5 2024 10:00 z', 'Jan 5 2024 10:00 +1', 'Jan 5 2024 10:00 -12345',
    'Jan 5 2024 10:00:00.5', 'Jan 5 2024 10::', 'Jan 5 2024 10:: 11::', '10:00 Jan 5 2024', 'Jan 5', '5', '12', '13',
    '0', '00', '99', '100', '1 2 3', '1 2 3 4', '50 1 1', '49 1 1', '2024-01-05 10:00', '2024-01-05 garbage', 'garbage 2024',
    'garbage2024', 'x 1 2', '(comment) Jan 5 2024', 'Jan 5 2024 (comment', 'Jan 5 2024 )', 'Jan (5) 2024', 'Jan 5 2024 +',
    'Jan 5 2024 10:00 +', 'Tue', 'today', 'now', 'after a', 'Q1 2024', '2024 Q1', 'Jan-5-2024', '5-Jan-2024', 'Jan-2024',
    '2024-Jan-05', 'T10:00', '2024T10:00', '2024-01-05T', '2024-01-05t10:00', '2024-01-05T10', '2024-01-05T10:00Z junk',
    '2024-01-05T10:00:00.Z', '1e3', '0x10', '-1', '+1', '1.5', '2024.01.05', '05.01.2024', '10.30', '10:30', '10:30:45.123',
    '10:30:45.1234', '10:30 am', '10:30am', '12 pm 1 1', 'Jan 1 2000 GMT+298262:', 'Jan 1 2000 GMT+596524:',
    'Jan 1 2000 GMT+400000:', 'Jan 1 2000 GMT+99:99', 'Jan 1 2000 UT', 'Jan 1 2000 UTC', 'Jan 1 2000 utc+1', 'Jan 1 2000 CDT',
    'Jan 1 2000 MST', 'Jan 1 2000 10:00 AMX', 'Mayday 1 2000', 'Sept 1 2000', 'Decem 1 2000', 'ja 1 2000', 'mar 1 2000',
    '2000 1 Jan', '1 Jan 31', 'Jan 1 2000\u00a0', '\u3000Jan 1 2000', 'Jan\u20281 2000', 'Jan 1 2000\u0000junk',
    'Jan 1 2000 \u00e9t\u00e9', '\u00c9 Jan 1 2000', '[Jan] 1 2000', '_Jan 1 2000', 'Jan_1_2000', 'Jan 1 2000 at 10',
    '2024-W01', '2024-001', '1000000000 1 1', '123456789012 1 1', '0000000001 1 1', 'Jan 1 -5', 'Jan 1 2000 -05:00',
    '2000-01-01T00:00:00.000+24:00', '2000-01-01T00:00:00.000+23:59', '2000-01-01T00:00+2359', '2000-01-01T00:00+2459',
    '2019-03-31T01:30', '2019-03-31T02:30', '2024-10-06T02:30', '2024-04-07T02:30', '2024-03-10T02:30', '2024-11-03T01:30',
    '10/6/2024 2:30', '4/7/2024 2:30', '3/10/2024 2:30', '11/3/2024 1:30', 'Oct 6 2024 2:30 am',
  ];
  for (const s of fixed) parse(s);
  const pieces = ['2024', '01', '1', '5', '12', '31', '-', '/', ':', '.', ' ', 'T', 'Z', '+', 'GMT', 'Jan', 'pm', 'am', ',', '(',
    ')', '10', '00', '000', 'x', 'UTC', '+05:30', '99', '1999', '0', '\u00a0', 'mar', 'est', '24', '60', '123456'];
  for (let i = 0; i < 1200; i++) {
    let s = '';
    for (let k = int(1, 9); k > 0; k--) s += pick(pieces);
    parse(s);
  }
  return cases;
}

if (process.argv[2] === '--zone') {
  process.stdout.write(JSON.stringify(casesIn(process.env.TZ)));
} else {
  const self = fileURLToPath(import.meta.url);
  const all = [];
  for (const zone of ZONES) {
    const out = execFileSync(process.execPath, [self, '--zone'], { env: { ...process.env, TZ: zone }, maxBuffer: 1 << 30 });
    all.push(...JSON.parse(out));
  }
  const target = join(dirname(self), '..', 'internal', 'jsdate', 'testdata', 'cases.json');
  writeFileSync(target, '[' + all.map((c) => JSON.stringify(c)).join(',\n') + ']\n');
  console.log(`${all.length} cases`);
}
