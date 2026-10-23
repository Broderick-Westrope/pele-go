# raster

Mermaid source to pixels in pure Go: [pele-go](https://github.com/Broderick-Westrope/pele-go)
lays the diagram out as SVG, and resvg, compiled to WebAssembly and run by wazero,
rasterizes it. It needs no cgo, browser or separate process, so
it builds with `CGO_ENABLED=0`.

Labels are measured in the same DejaVu Sans faces that are compiled into `resvg.wasm`, so
they fit their boxes when drawn.

## Install

```sh
go get github.com/Broderick-Westrope/pele-go/raster
```

Go 1.26 or later. This is its own module so that wazero stays out of pele-go's root module.

## Use

Layout and rasterizing are separate steps, so you can size the image from the layout
before you draw it:

```go
r, err := raster.NewRenderer()
if err != nil {
	return err
}
res, err := r.Render(ctx, "flowchart LR\n  A --> B", raster.Options{MaxWidth: 900})
if err != nil {
	return err
}

rast, err := raster.NewRasterizer(ctx, cacheDir)
if err != nil {
	return err
}
defer rast.Close()
img, err := rast.Rasterize(res, raster.Theme{"pele-bg": "#ffffff", "pele-fg": "#000000"},
	raster.RasterOptions{Scale: 2})
if err != nil {
	return err
}
// img is an *image.RGBA; encode it however you like, e.g. with image/png.
```

- `Renderer` is safe for concurrent use. It refuses sources over `MaxSourceBytes`, stops
  layout after 5 seconds with `ErrRenderTimeout`, and turns images off and responsive
  sizing off, since the source may be untrusted.
- `Rasterizer` is not safe for concurrent use, and you own its lifecycle. Creating one
  compiles resvg (a few hundred milliseconds, about 75 MB of short-lived garbage) unless
  `cacheDir` holds code an earlier `Rasterizer` compiled, in which case it loads in
  milliseconds. The cache holds native code, so the directory must be writable only by you.
  `Close` frees everything it holds.
- `Rasterize` caps the image at `RasterOptions.MaxPixels` (default `MaxPixels`), shrinking
  the scale only as far as it must. resvg running past 10 seconds returns
  `ErrRasterTimeout`, wrapped.
- `Theme` maps Pele's CSS variables, without the leading `--`, to values. resvg can't
  resolve CSS variables, so `Rasterize` substitutes them first. `ResolveVars` does the same
  for SVG you show elsewhere, and `FontFaces` returns the fonts to embed alongside it.

## Upgrading resvg or the fonts

The fonts are embedded twice: as `fonts/*.ttf`, which pele-go measures labels with, and
compiled into `internal/resvg/resvg.wasm`, which draws them. They must be the same files,
or labels overflow their boxes. They must also match `../fontmeasure/testdata/*.ttf`,
which `TestFontsMatchFontmeasureTestFonts` checks.

- **resvg:** `resvg.wasm` comes from
  [go-resvg](https://github.com/boolean-maybe/go-resvg) v0.0.2 (resvg 0.44). Rebuild it from
  the Rust crate in that repository for the `wasm32-wasip1` target. The Go side depends only
  on its exports (`wasm_alloc`, `wasm_free`, `render`) and the 20-byte result header
  described in `internal/resvg/resvg.go`.
- **Fonts:** change `fonts/`, `resvg.wasm` and `../fontmeasure/testdata` in the same change.
- Run `go test ./...` here and look at the output with `MERMAID_TEST_OUT=<dir>`, which
  writes a PNG of each sample diagram and `flowchart.svg`.

### Release order

This module requires a tagged pele-go, so a change to the fonts or to `fontmeasure` takes
three steps, and one pull request can't do all of them:

1. Tag the root module (`vX.Y.Z`).
2. Bump `github.com/Broderick-Westrope/pele-go` in this module's `go.mod` to that tag.
3. Tag this module (`raster/vX.Y.Z`).

## Licence

MIT, see [LICENSE](LICENSE). `internal/resvg/resvg.wasm` is resvg and go-resvg, Apache-2.0,
see [internal/resvg/LICENSE](internal/resvg/LICENSE). The DejaVu fonts are under the
Bitstream Vera licence, see [fonts/LICENSE](fonts/LICENSE). Both are compiled into your
binary.
