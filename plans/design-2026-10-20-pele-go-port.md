# Pele Go Port Design Spec

**Problem:** Anvil renders Mermaid by running Pele 1.7.0 (`pele.global.js`, 683 KB) in goja. Profiling shows Pele in goja costs about 48 MB at load, settling to about 25 MB, and is the largest share of the 60–70 MB Anvil adds while diagrams are rendering (with the wazero compilation cache). The compilation cache can't reduce it. Separately, there is no Go-native Mermaid renderer that other Go programs can use without a JS runtime.

**Goal:**
- Primary (Anvil memory): Anvil renders diagrams with a pure-Go port of Pele, removing goja Pele (about 25–48 MB) from the render path, so the inline rendering-state increase is set mostly by resvg and the image.
- Secondary (reusable library): `github.com/Broderick-Westrope/pele-go` (package `pele`) is a public, dependency-free Go module that produces byte-identical SVG to a pinned upstream Pele release and keeps up with later releases.

**Scope:**
- In:
  - All 33 diagram types in Pele 1.7.0.
  - `core`, `detect`, `preprocess`, `errors`, `svg/`, `text/`, `util/`, `layout/` and `diagrams/common`.
  - The built-in Arial-table measurer.
  - A `fontmeasure` subpackage.
  - The parity oracle, input extraction and fuzzing.
  - Anvil cutover and a superseding ADR.
- Out:
  - Browser-only parts (`mount`, `zoom`, `lazy`, `window`, `enableZoom`).
  - A public `Parse()` and typed models.
  - Replacing resvg (possible follow-up spec).
  - Changing the diagrams Anvil shows, or how.
  - Following upstream `main` during the port.
- Anvil changes nothing until all 33 types pass parity. There is no partial rollout and no goja fallback.

**Constraints:**
- "Pele 1.7.0" means the `1.7.0` release tag, commit `8603bf3313add9143cd417015d4a6bd6c522dada`, for source, tests and the IIFE bundle. Upstream `main` was already two commits ahead (including `src/svg/root.ts`). The oracle bundle is built from that commit and its checksum recorded.
- Byte-identical to Pele 1.7.0 for the same source, options, `now`, time zone and `TextMeasurer`: SVG string, `width`, `height` and `links` on success; error code and message on failure. There are two contracts:
  - **Internal (lossless):** `internal/render.RenderWTF8` returns the SVG as WTF-8. Converted to UTF-16, it must equal the JS output code unit for code unit, including lone surrogates.
  - **Public (UTF-8):** `pele.Render` returns `RenderWTF8`'s output with each lone surrogate replaced by U+FFFD. It must equal the JS output encoded the same way, the same as `TextEncoder` does.

  The oracle checks both contracts on every case.
- `CGO_ENABLED=0`. The root `pele` package has no dependencies outside the standard library. `fontmeasure` may depend on `golang.org/x/image`. goja is a test-only dependency.
- `Render` is safe for concurrent use. Package-level state is limited to:
  - the diagram registry, written only from `init` (through `pele.Register`) and read-only afterwards; calling `Register` after the first `Render` panics;
  - read-only tables built once with `sync.OnceValue` (metric tables, unicode tables).

  Nothing else at package level changes after initialisation.
- Files map one-to-one to upstream `src/` files so release diffs can be ported mechanically. Intentional differences (context checks, depth counter, concurrency changes) are marked with a short comment.
- MIT licence with upstream Pele's copyright notice preserved.

**Success Criteria:**
- [ ] All three oracle input sets match Pele 1.7.0 exactly:
  - (1) every source in `tests/corpus/*.json`, rendered with default options and a fixed `now`;
  - (2) every public `render`/`parse` call made while running the pinned vitest suite under `tests/compat/**` and `tests/render/**`;
  - (3) a Go fuzz target per diagram type, seeded from (1) and (2) and run with a fixed `now`, runs 60 s per type on PRs that touch that type and longer nightly, with no mismatch.
  - Inputs within 10% of the Go depth limit are excluded.
- [ ] Capture contract for set (2): a vitest setup file wraps the public `render` and `parse` from `src/core.ts` and writes one JSON record per call:
  - source;
  - every serialisable option;
  - the measurer, recorded as `"default"` (none given), `"metrics"` (upstream's exported `metricsMeasurer`, used by many render tests) or `"custom"`;
  - `now`, defaulted to a fixed instant when unset;
  - icon-resolver results, recorded as a name-to-result lookup table that the Go replay supplies as its resolver;
  - the call's outcome.

  Replay maps `"default"` and `"metrics"` to the Go Arial-table measurer. Calls with a custom JS measurer or another option that can't be serialised are dropped. Their count goes in the corpus manifest, and extraction fails if the count is above the recorded baseline.

  `render(model)` calls (rendering a model returned by `parse`, for example `tests/render/all-types.test.ts`, which renders parsed models under limits that would reject the source) are recorded as one case with the source and options from the `parse` call that produced the model, the separate `render` options, and the render outcome. The Go replay runs an internal parse-then-render path with those two option sets, since `Parse` isn't public. `parse` calls on their own record their outcome as success or the error code and message. `render` calls given an object that `parse` didn't return (for example `render({...parse(src)})`, which upstream rejects with `TypeError`) have no Go equivalent, because `Parse` isn't public, so they're excluded and counted in the manifest.

  Compat adapters that call internal parsers directly (for example `tests/compat/flowchart/adapter.ts`) aren't captured; they're covered by the ported model tests.
- [ ] Each oracle case runs in two time zones (UTC and a non-zero zone with daylight saving, for example `Australia/Sydney`), fixed on both sides.
- [ ] On set (1), the Go default Arial-table measurer is also compared against upstream's own JS default, not only through the shared measurer, so bugs in the Go default measurer can't hide.
- [ ] Model-level assertions from `tests/compat/**` are ported as package-internal Go tests.
- [ ] `go test -race ./...` passes, including a test that renders different diagrams concurrently.
- [ ] A render with a context cancelled before or during layout returns a `*pele.Error` with code `Canceled` that wraps `ctx.Err()`, and no goroutine keeps running afterwards.
- [ ] Inputs nested past the fixed depth limit return code `limit` with Pele's message ("Diagram is nested too deeply.") and never overflow the stack.
- [ ] Library: rendering every corpus diagram with the `fontmeasure` measurer keeps Go heap in use, after `runtime.GC()`, within 2 MB of the pre-render baseline. Peak heap during the largest corpus diagram is reported in the benchmark output.
- [ ] Anvil, inline rendering: the rendering-state increase is 25 MB or less, measured the same way, on the same fixtures and with the compilation cache as the 2026-10 profile. The budget is about 18.5 MB for resvg and the image, plus headroom. A benchmark of this is run straight after the `info` tracer to confirm the headroom before most of the port happens.
- [ ] Anvil, drill-in: peak memory is no higher than today. The 64 MB image dominates it and no absolute budget is set.
- [ ] Anvil: median and p99 source-to-SVG latency over the oracle corpus are no worse than goja Pele.
- [ ] Anvil: `internal/mermaid/pele.global.js`, `PELE_LICENSE`, the goja `Renderer` (including the interrupt and timer code), and `measure.go` are deleted. Rendering goes through `pele-go` with the `all` package and `fontmeasure` using the embedded DejaVu fonts. The idle teardown remains for resvg only.
- [ ] Anvil: tests that check behaviour (raster, vars, export, `ui/model/mermaid_test.go`, and the parts of `pele_test.go` that test rendering results and error handling) pass without changes to their assertions. Tests of deleted internals (goja loading, interrupt timer, measurer) are replaced by context-deadline tests. `MERMAID_TEST_OUT` output is identical to before the cutover.
- [ ] Anvil: a new ADR supersedes ADR 0004's Pele/goja decision. The resvg decision stays.

**Design Decisions:**
- **The port is driven by Anvil memory, not binary size.** goja stays in Anvil for workflows. The saving is the per-renderer goja runtime and the loaded bundle (25–48 MB).
- **All 33 types before cutover (no partial rollout and no goja fallback).** This gives one clean switch. Declined: dropping goja once the top types are done, with code-block fallback for the rest (lower fidelity); keeping goja as a lazy fallback (brings the memory cost back and means two code paths).
- **Byte-identical parity, not visual or behavioural equivalence.** Pele promises the same SVG for the same input and measurer, so parity is one automatic string comparison and "done" can be checked objectively. It also keeps upstream release diffs cheap to port.
- **Pin to Pele 1.7.0, then follow releases as diffs.** Each upstream release becomes one Go change that also bumps the pinned oracle. Anvil's `pele.global.js` stays at 1.7.0 until cutover. The oracle has its own test-only copy. Declined: tracking `main` (the finish line keeps moving); freezing as a fork (loses Mermaid fixes).
- **Separate public repo `github.com/Broderick-Westrope/pele-go`, package `pele`.** Library users don't pull in Anvil's dependencies, and the oracle weight lives with the library. Anvil uses a `replace` directive during local development. Declined: `internal/` in Anvil (has to be moved before shipping); `pkg/` in Anvil.
- **Minimal v1 API:**
  - `Render(ctx context.Context, src string, opts Options) (Result, error)`
  - `DetectType(src) (DiagramType, bool)` and `Supports(src) bool`
  - a `TextMeasurer` interface, with the Arial-table measurer as the default
  - a typed `*Error{Code, Type, MermaidType, ...}`
  - `Result{SVG, Width, Height, Links, Type, MermaidType}`

  `Options` covers every field in JS `RenderOptions` (`src/types.ts`): `Measurer`, `FontFamily`, `FontFamilyMono`, `FontSize`, `IDPrefix`, `Responsive`, `MaxWidth`, `AutoDirection`, `DirectionBreakpoint`, `Padding`, `Limit`, `OutputLimit`, `MaxEdges`, `LinkSchemes`, `ImageSchemes`, `Links`, `Images`, `LinkRel`, `Icons`, `Config` and `Now`, plus a Go-only `Location *time.Location` (default `time.Local`) standing in for the JS process time zone used by gantt.

  Wherever JS treats "unset" differently from a zero or empty value, the Go field is a pointer, or a nil slice versus an empty slice. Examples: `Responsive`, `AutoDirection`, `Links`, `Images`, `LinkRel` (an empty string omits `rel`), `Padding`, `FontSize`. `Now` unset means the current time, as upstream does.

  Diagram types live in `diagrams/<type>` subpackages and register with `pele.Register`. An `all` package registers every type. Declined: a public `Parse` with typed models (a large API to lock in, not needed by Anvil, can be added later without breaking anything); a single package with no registry (doesn't match upstream's layout).
- **Context cancellation, checked between phases and in the iterative loops of `layout/`.** Go can't preempt a goroutine, so without these checks a runaway layout couldn't be stopped. The checks don't change output.
- **A fixed depth counter replaces JS `RangeError`.** JS overflow depends on the engine's stack size, so the oracle excludes inputs near the limit.
  - *Note (2026-10-21):* the counter was found unnecessary and none was added. Upstream walks nested input without recursion that input depth can overflow, V8 renders 100,000-deep sources, and Go's growable stacks let the port do the same, so deep inputs match V8 with no limit (`TestDeepNestingRenders`). RangeErrors that upstream does throw (for example `String.fromCodePoint` given a fraction) still map to `limit`. The depth-limit success criterion and the near-limit exclusion in set (3) no longer apply.
- **No mutable package-level state beyond the init-only registry and the `sync.OnceValue` tables listed under Constraints.** The text-width cache becomes a per-render wrapper around the measurer, and the icon resolver is passed through a render context instead of `withIcons` setting a global. `forgetTextWidths` is removed. Custom `TextMeasurer` implementations must be safe for concurrent use, which is documented. Declined: a shared cache behind a mutex (memory with no limit).
- **Measurers:** the root package ships the byte-exact Arial-table `metricsMeasurer`. `fontmeasure.New(regular, bold, italic, boldItalic []byte)` builds an sfnt-based measurer from fonts the caller supplies, generalised from Anvil's `internal/mermaid/measure.go`. No fonts are embedded in the library. Declined: embedding DejaVu (adds about 500 KB to every user's binary).
- **Strings:** Go `string` at the API boundary. The preprocessor, lexers, scanners and measurer work on a UTF-16 `[]uint16` source type with JS-exact helpers in `internal/jsstr`, so offsets, limits, slices and surrogate handling match upstream. Values leaving the lexers become Go strings once. Declined: rewriting every index operation for UTF-8 (hundreds of chances to diverge); UTF-16 everywhere (hard to read, no benefit).
- **Lone surrogates:**
  - Internal Go strings use WTF-8, so lone surrogates survive the UTF-16 to UTF-8 conversion without loss. A lone surrogate can come from YAML `\u` escapes in config, for example (`src/util/yaml.ts`).
  - Length checks that upstream does in UTF-16 (`limit` on input, `outputLimit` on the SVG in `src/core.ts`) count UTF-16 code units through `jsstr`.
  - `Result.SVG` follows the public UTF-8 contract under Constraints. `RenderWTF8` keeps the lossless form for the oracle and future callers who need it.
- **JS-exactness helpers form the foundation layer:**
  - `jsstr`
  - `jsmath.Round`: JS rounds half-way values up, unlike `math.Round`.
  - `jsnum.String`: JS number-to-string, plus `toFixed`.
  - `omap`: insertion-ordered Map and Set, replacing about 300 JS `Map`/`Set` uses.
  - stable sorting only (`slices.SortStableFunc`).
  - `jsstr.Compare`: string comparison by UTF-16 code unit, as JS `<` and default `sort()` work (for example `src/diagrams/journey/db.ts`). This is not Go's byte order.
  - `jsobj`: JS object-key enumeration order (integer-like keys ascending, then string keys in insertion order). It's separate from `omap` and used wherever upstream walks plain objects (for example `src/diagrams/sequence/db.ts`).
  - Each helper gets targeted fixtures, checked against goja, before any diagram type is ported: half-way rounding, number formatting at exponent boundaries, astral and lone-surrogate strings, integer-like object keys, gantt date overflow across time zones.
  - `jsdate`: the subset of JS `Date` semantics gantt uses. Gantt has 31 `new Date`/`Date.*` uses, including month and day overflow and local-time behaviour.

  The roughly 30 regex lookaheads and `\p{}` classes are rewritten by hand for RE2.
- **Port order (each type is gated on its full oracle subset passing):**
  1. Foundation (JS helpers, `svg/`, `text/`, `util/`, `diagrams/common`, `core`, `detect`, `preprocess`) and the harness (goja oracle running the pinned `pele.global.js` with the same Go measurer, input extraction script, fuzz scaffolding).
  2. A tracer: `info`, end to end.
  3. In parallel: the 25 types that only depend on `common` (one worktree and one PR per type), and on the critical path, `layout/` followed by flowchart.
  4. The 7 types that depend on layout or flowchart: class, er, state, requirement, usecase, agentflow, swimlane. Then the Anvil cutover.

  Declined: a vertical flowchart slice first (keeps easy work waiting); smallest-first (leaves the riskiest work until last).

**Context Files:**
- Upstream Pele 1.7.0: `src/core.ts`, `src/svg/builder.ts` (`num`), `src/text/measurer.ts`, `src/text/metrics.ts`, `src/layout/{layered,compound,lanes}.ts`, `src/diagrams/*/`, `src/diagrams/gantt/dates.ts`, `tests/corpus/*.json`, `tests/compat/**`, `tests/render/**`, `AGENTS.md`.
- Anvil (`github.com/Broderick-Westrope/anvil`): `docs/adr/0004-mermaid-rendering.md`, `internal/mermaid/{pele.go,measure.go,raster.go,vars.go,export.go}`, `internal/mermaid/resvg/resvg.go`, `internal/mermaid/fonts/`, `internal/ui/model/mermaid_render.go`, `plans/completed/design-2026-10-20-mermaid-diagrams.md`.
- Profile: the 2026-10 per-instance memory table (Pele in goja about 48 MB at load and about 25 MB settled; resvg machine code about 6 MB; resvg working memory about 10 MB; image about 2.5 MB inline).
