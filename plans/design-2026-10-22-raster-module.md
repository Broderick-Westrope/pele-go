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
- In:
  - Create the nested module `raster/`, with its own `go.mod`, in this repo.
  - Move these files from `anvil/internal/mermaid` into it, with their tests:
    `pele.go` (`Renderer`, `Options`, `Result`, `MaxSourceBytes`,
    `ErrSourceTooLarge`, `ErrRenderTimeout`), `raster.go` (`Rasterizer`,
    `RasterOptions`, `MaxPixels`), `vars.go` (`Theme`), `fonts.go`, `fonts/`
    and `resvg/` (Go wrapper, `resvg.wasm`, `LICENSE`).
  - Switch Anvil to import the module, keep `HTMLDocument` (`export.go`) in
    Anvil, and delete Anvil's moved copies.
  - Move ADR 0004's "Upgrading" steps for resvg and fonts into the module's
    README. Anvil's ADRs point there.
  - Tag the first release as `raster/v0.1.0`.
- Out:
  - Any API change, new behaviour or new options. The proxy adds PNG encoding
    and its own lifecycle on top.
  - PNG encoding inside the module.
  - Any change to pele-go's root module.

**Constraints:**
- **No performance regression in Anvil.** Move the API exactly as it is:
  - Layout (`Renderer.Render`) and rasterizing (`Rasterizer.Rasterize`) stay
    separate calls. Anvil runs layout first to work out the fit.
  - `Rasterize` still returns `*image.RGBA`.
  - The caller still owns the `Rasterizer`: it constructs it with `cacheDir`
    and calls `Close`. Anvil's lazy load, 20 s idle teardown and shared
    wazero compilation cache (ADR 0004 and 0005) keep working unchanged.
- **Root module stays stdlib-only.** `wazero` is required only by
  `raster/go.mod`. Consumers of `github.com/Broderick-Westrope/pele-go` don't
  pull it in.
- **`raster/go.mod` declares `go 1.26.0`,** the same as the root module. The
  proxy builds on `golang:1.26` with `CGO_ENABLED=0`.
- **Fonts and `resvg.wasm` change together.** They live in one package, and the
  README says so.
- **Licences travel with their files:** resvg (Apache-2.0), DejaVu/Bitstream
  Vera, and pele-go's MIT.

**Success Criteria:**
- [ ] `go test ./...` passes in `raster/` with every moved test, including the
  resvg tests and the label-sizing test.
- [ ] Anvil's `go test ./...` and its lint/typecheck pass against the module.
  The repo's `go.work`, or a `replace` directive, is used during development;
  the final Anvil commit requires the tag `raster/v0.1.0` with no `replace`.
- [ ] Every file `MERMAID_TEST_OUT` writes from Anvil is byte-identical before
  and after the switch, run back to back (ADR 0006's method; Gantt marks the
  current time).
- [ ] `scripts/anvil-memory` added RSS and Go heap in use, and source-to-SVG
  median/p99 over the oracle corpus, are within run-to-run noise of ADR
  0006's figures, measured before and after.
- [ ] `go list -m all` in the root module shows no `wazero`.
- [ ] Building a `CGO_ENABLED=0` binary that imports `raster` with Go 1.26
  succeeds.
- [ ] Anvil's `internal/mermaid` holds only `export.go` and its test, and the
  binary size changes by less than 100 KB.

**Design Decisions:**
- **Module name `raster`, not `png`.** It returns RGBA pixels and also owns the
  font-matched layout wrapper. PNG encoding is the caller's choice.
- **A nested module rather than a package in the root module.** This keeps
  pele-go's dependency-free promise (ADR 0006) and lets the raster pipeline
  version on its own.
- **A nested module rather than a new repo.** The fonts must match pele-go's
  `fontmeasure` behaviour, and pele-go's oracle corpus is the natural test
  input. Keeping them in one repo means one PR can change both.
- **`HTMLDocument` stays in Anvil.** It names the font family
  `Anvil DejaVu Sans`, and moving or renaming it would change Anvil's exported
  pages. To support it, the module exports `ResolveVars(svg string, theme
  Theme) (string, error)` and `FontFaces()`, which returns the embedded faces
  with their CSS weight and style. These are the only two additions to the
  surface, and they exist because `export.go` already uses the unexported
  versions.
- **The module, not callers, sets the layout options** (responsive off, images
  off, `MaxWidth` always set). Both callers render model-written source, so
  images must stay off for both.
- **Declined: a one-shot `Render(source) -> PNG` helper.** It would merge
  layout and raster, hide the engine's lifecycle, and add an encode that
  Anvil doesn't need. Each of those is a performance regression for Anvil.
  The proxy writes its own five lines of glue.
- **Declined: copying the files into the proxy.** That gives two copies of the
  font/resvg invariant and two resvg upgrade paths.
- **Ownership risk acknowledged.** A company service will depend on a personal
  repo. The Go checksum database pins each tag, but whoever owns the repo
  controls future versions. If Eucalyptus Security objects, the fallback is
  to mirror the module under `eucalyptusvc`, which is a rename for the proxy.

**Context Files:**
- `anvil/internal/mermaid/{pele,raster,vars,fonts,export}.go` and tests
- `anvil/internal/mermaid/resvg/resvg.go`, `resvg.wasm`, `LICENSE`
- `anvil/internal/mermaid/fonts/`
- `anvil/internal/ui/model/mermaid_render.go` (lazy load, idle teardown,
  two-pass fit) and `graphics.go` (theme mapping)
- `anvil/docs/adr/0004-mermaid-rendering.md`, `0005-cross-instance-sharing.md`,
  `0006-pele-go.md`
- `pele-go/fontmeasure/`, `pele-go/scripts/anvil-memory`,
  `pele-go/internal/oracle/testdata/corpus`
- Consumer spec: `agent.credential.proxy/plans/design-2026-10-22-slack-diagrams.md`
