# Raster Module Design Spec

**Problem:** Anvil's Mermaid-to-pixels pipeline (`internal/mermaid` in
`Broderick-Westrope/anvil`) is the only proven way to rasterize pele-go
output without cgo, a browser, or a separate process. The Eucalyptus agent
credential proxy needs the same pipeline to post Mermaid diagrams into Slack
as PNGs, but `internal/` packages can't be imported from another module.
Copying the files would give two copies of the invariant that is easiest to
break: pele-go must measure labels with the same DejaVu Sans faces that are
compiled into `resvg.wasm`, or labels overflow their boxes.

**Goal:** A nested Go module, `github.com/Broderick-Westrope/pele-go/raster`,
holds the pipeline. Anvil imports it instead of its own copy, with no change in
output, memory or latency. The proxy can then import a tagged version.

**Scope:**
- **In:**
  - Create the nested module `raster/` with its own `go.mod`, `LICENSE` (MIT)
    and `README.md`.
  - Move these from `anvil/internal/mermaid`, with their tests:
    - `pele.go`: `Renderer`, `Options`, `Result`, `MaxSourceBytes`,
      `ErrSourceTooLarge`, `ErrRenderTimeout`.
    - `raster.go`: `Rasterizer`, `RasterOptions`, `MaxPixels`.
    - `vars.go`: `Theme`.
    - `fonts.go` and `fonts/`.
    - `resvg/` as `raster/internal/resvg`, with its Go wrapper, `resvg.wasm`
      and `LICENSE`.
  - Add a benchmark harness to Anvil before the switch (see "Proving no
    regression").
  - Switch Anvil to the module, keep `HTMLDocument` (`export.go`) in Anvil,
    and delete Anvil's moved copies.
  - Add a CI job for `raster/` in pele-go.
  - Move ADR 0004's "Upgrading" steps for resvg and fonts into
    `raster/README.md`, adding the release order below. Anvil's ADRs point
    there.
  - Tag `raster/v0.1.0`.
- **Out:**
  - New behaviour or options.
  - PNG encoding.
  - Any change to pele-go's root module.
  - A committed `go.work` in either repo.

**Constraints:**
- **No performance regression in Anvil.** The API moves as it is:
  - Layout (`Renderer.Render`) and rasterizing (`Rasterizer.Rasterize`) stay
    separate calls, because Anvil lays out first to compute the fit.
  - `Rasterize` still returns `*image.RGBA`.
  - The caller still owns the `Rasterizer`: it constructs it with `cacheDir`
    and calls `Close`. Anvil's lazy load, 20 s idle teardown and shared
    wazero compilation cache (ADR 0004 and 0005) are unchanged.
- **pele-go's root package stays dependency-free.** The root module already
  requires `goja` and `x/image` for tests and tooling. It must not gain
  `wazero`: only `raster/go.mod` requires it.
- **`raster/go.mod`:**
  - Declares `go 1.26.0`. The proxy builds on `golang:1.26` with
    `CGO_ENABLED=0`, and wazero v1.12.0 needs go 1.25.
  - Pins exactly what Anvil uses today: `pele-go v0.1.0` and
    `wazero v1.12.0`. Minimal version selection must not silently upgrade
    Anvil.
  - Adds `testify` as a test-only dependency, which the root module doesn't
    use.
- **Fonts and `resvg.wasm` change together.** The fonts are embedded in
  `raster` and the wasm in `raster/internal/resvg`. A test, together with the
  README, enforces the pairing (see Success Criteria).
- **Licences travel with their files:**
  - `raster/LICENSE` is MIT, because a module zip doesn't include the root
    `LICENSE`.
  - resvg is Apache-2.0, and the fonts are DejaVu/Bitstream Vera.
  - The moved code changes from Anvil's FSL-1.1-MIT to MIT. Brodie is its
    only author.

**Test split:**
- **Module tests:** the moved `pele_test.go`, `raster_test.go`, `vars_test.go`
  and `resvg_test.go`, with these changes:
  - `TestRasterizeDiagrams` and `TestRenderLabelDeclaringVariablesStaysSmall`
    call `HTMLDocument` today. In the module they assert on
    `ResolveVars(...)` instead: no `var(` remains, and the size stays
    bounded.
  - `MERMAID_TEST_OUT` keeps writing the PNGs and `flowchart.svg` from the
    module.
- **Anvil keeps `export_test.go`,** with its own copies of the fixtures it
  used from `raster_test.go` (`sampleDiagrams`, `testTheme`,
  `explodingVarsSVG`). It counts `raster.FontFaces()` instead of
  `fontFaces`, and keeps a label-declares-variables case for `HTMLDocument`.
- **Heap-growth tests** (`TestRenderAcrossGCsDoesNotGrow`,
  `TestRenderReleasesGrownMemory`) must be stable under `-race` in CI.
  Otherwise they skip under `-race` with a reason.

**New public surface (and nothing else):**
- `ResolveVars(svg string, theme Theme) (string, error)`: the unexported
  `resolveVars` without the `post` hooks. `HTMLDocument` needs it.
- `type FontFace struct { Data []byte; Weight, Style string }` and
  `FontFaces() []FontFace`. `Data` aliases the embedded bytes, and the
  documentation says callers must not modify it.
- `ErrRasterTimeout`, which is the same value as `internal/resvg`'s timeout
  error, so `errors.Is` keeps working.
  - Its doc comment calls it the resvg timeout, as opposed to
    `ErrRenderTimeout`, the layout timeout.
  - Anvil's `mermaid_render.go:81` changes from `resvg.ErrRenderTimeout` to
    `raster.ErrRasterTimeout`.
  - Anvil's local variables named `raster` (`mermaid_render.go:204,249`,
    `graphics_copy_test.go:83`) are renamed so they don't shadow the
    package.

**Proving no regression:**
- **Harness:** before the switch, add `anvil/internal/mermaidbench/` as a
  test-only package in its own commit. It uses only the exported API, so the
  after-run differs from the before-run only in its import path. It measures:
  1. Source-to-SVG median and p99 over pele-go's oracle corpus (path from
     `PELE_CORPUS`), at `MaxWidth` 900, with five passes after a warm-up.
     Sources that fail must fail identically in both builds, and their
     count is reported.
  2. `NewRasterizer` wall time from a warm compilation cache, over 20
     iterations.
  3. `Rasterize` wall time at Anvil's scale, 20 iterations each on `info`
     and every `sampleDiagrams` entry.
  4. Added RSS and Go heap in use after rendering and rasterizing `info` 20
     times with resvg loaded from a warm cache, keeping the last image. This
     is ADR 0006's method, reimplemented because `scripts/anvil-memory` no
     longer applies.

  The harness skips when `PELE_CORPUS` is unset, so Anvil's CI stays green.
- **Runs:**
  - Build `go test -c` binaries at the harness commit (A) and after the
    switch (B).
  - Run them interleaved, ABABABAB, on the same machine, so drift doesn't
    line up with the before/after split.
- **Pass:**
  - For metrics 1–3, benchstat reports no statistically significant
    regression (p < 0.05), and the median of per-run medians is within ±5%.
  - Added RSS is within ±2 MB.
  - Heap in use and p99 latency are recorded, not gated.
  - When B loads after A, the compilation cache directory gains no new
    entries. The wasm bytes and the wazero version are unchanged, so the
    cache must still hit.

**Success Criteria:**
- [ ] `go vet`, staticcheck and `go test -race ./...` pass in `raster/`, in a
  new pele-go CI job (`working-directory: raster`,
  `go-version-file: raster/go.mod`).
- [ ] The same CI job builds a test binary with `GOTOOLCHAIN=go1.26.0
  CGO_ENABLED=0`.
- [ ] A test fails if the SHA-256s of `raster/fonts/*.ttf` and
  `fontmeasure/testdata/*.ttf` differ. It skips with a reason when
  `../fontmeasure` is absent, for example when the module is tested from
  the module cache.
- [ ] `go list -m all` in pele-go's root module shows no `wazero`.
- [ ] Anvil's `go test ./...` and lint pass against `raster/v0.1.0` with no
  `replace`. A temporary `replace` is used only during development.
- [ ] Output equivalence: the union of files that `MERMAID_TEST_OUT` writes
  from `raster` and from Anvil after the switch matches Anvil's set before
  the switch, name for name and byte for byte. The runs are back to back,
  because Gantt marks the current time.
- [ ] The harness passes the thresholds above, and the numbers are recorded
  in Anvil's PR.
- [ ] Anvil's `internal/mermaid` holds only `export.go` and its test, and
  the binary size changes by less than 100 KB.
- [ ] Anvil's ADRs 0004 and 0006 and `AGENTS.md` (lines 49, 219 and 239
  describe `internal/mermaid`) point at the module.

**Design Decisions:**
- **The module is named `raster`, not `png`.** It returns RGBA, and PNG
  encoding is up to the caller.
- **A nested module, not a package in the root module.** This keeps wazero
  out of the root module's graph and lets the raster pipeline version on its
  own.
- **A nested module, not a new repo.** The fonts must match `fontmeasure`'s
  test fonts, and the hash test only works with both in one repo.
- **`resvg` becomes internal.** Its ABI assumptions shouldn't become semver
  surface for a company consumer, and the one error Anvil checks is
  re-exported.
- **`HTMLDocument` stays in Anvil.** It names the font family
  `Anvil DejaVu Sans`, and moving it would change Anvil's exported pages.
- **The module sets the layout options** (responsive off, images off,
  `MaxWidth` always set). Both callers render model-written source.
- **Release order when fonts or fontmeasure change:**
  1. Tag the root.
  2. Bump `raster/go.mod`.
  3. Tag `raster/`.

  One PR can't do all three, and the README documents the order.
- **Declined: a one-shot `Render(source) -> PNG` helper.** It would merge the
  two steps, hide the lifecycle and add an encode, and each of those is a
  regression for Anvil.
- **Declined: copying the files into the proxy.** That means two copies of
  the invariant and two upgrade paths.
- **Ownership risk acknowledged.** The proxy's Dockerfile sets
  `GOPRIVATE=github.com`, so it skips both the checksum database and the
  module proxy. Only the proxy's `go.sum` pins the module, and builds fetch
  straight from the personal repo. A deleted or renamed repo breaks the
  proxy's builds. The fallback is a mirror under `eucalyptusvc`, which is a
  rename for the proxy. Whether to do that now is the Security reviewer's
  call (see the proxy spec).

**Context Files:**
- `anvil/internal/mermaid/{pele,raster,vars,fonts,export}.go` and tests
- `anvil/internal/mermaid/resvg/resvg.go`, `resvg.wasm`, `LICENSE`
- `anvil/internal/mermaid/fonts/`
- `anvil/internal/ui/model/mermaid_render.go` (lazy load, idle teardown,
  two-pass fit, `resvg.ErrRenderTimeout` at :81) and `graphics.go`
- `anvil/docs/adr/0004-mermaid-rendering.md`,
  `0005-cross-instance-sharing.md`, `0006-pele-go.md`
- `pele-go/fontmeasure/testdata/`, `pele-go/.github/workflows/ci.yml`,
  `pele-go/internal/oracle/testdata/corpus`
- Consumer spec:
  `agent.credential.proxy/plans/design-2026-10-22-slack-diagrams.md`
