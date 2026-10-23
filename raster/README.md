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

// The Rasterizer keeps this context for its whole life.
rast, err := raster.NewRasterizer(context.Background(), cacheDir)
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

- `Renderer` is safe for concurrent use. It refuses sources over `MaxSourceBytes` and stops
  layout after 5 seconds with `ErrRenderTimeout`. It turns images off, since the source may
  be untrusted and a browser showing the SVG would fetch them, and responsive sizing off,
  so the SVG has a fixed size to rasterize.
- `Rasterizer` is not safe for concurrent use, and you own its lifecycle. Creating one
  compiles resvg (a few hundred milliseconds, about 75 MB of short-lived garbage) unless
  `cacheDir` holds code an earlier `Rasterizer` compiled, in which case it loads in
  milliseconds. The cache holds native code, so the directory must be writable only by you.
  It keeps the context you create it with, and every `Rasterize` fails once that context
  is done, so give it a long-lived one. `Close` frees everything it holds either way.
- The image's background is transparent. Draw it over the theme's `pele-bg` yourself if you
  need an opaque one, for example before posting it somewhere with a different background.
- `Rasterize` caps the image at `RasterOptions.MaxPixels` (default `MaxPixels`), shrinking
  the scale only as far as it must. resvg's memory is capped at 256 MB, room for at least
  16M pixels; past that cap the render fails with a wasm error. resvg running past 10
  seconds returns `ErrRasterTimeout`, wrapped.
- This module requires pele-go itself, and is tested against that version only. Take
  pele-go upgrades through raster releases rather than bumping
  `github.com/Broderick-Westrope/pele-go` in your own `go.mod`.
- `Theme` maps Pele's CSS variables, without the leading `--`, to values. resvg can't
  resolve CSS variables, so `Rasterize` substitutes them first. `ResolveVars` does the same
  for SVG you show in a browser, though unlike `Rasterize` it leaves `color-mix()` for the
  browser to resolve. `FontFaces` returns the fonts to embed alongside it.

## Upgrading resvg or the fonts

Paths starting `../` are in the pele-go repository, outside this module.

The fonts are embedded twice: as `fonts/*.ttf`, which pele-go measures labels with, and
compiled into `internal/resvg/resvg.wasm`, which draws them. They must be the same files,
or labels overflow their boxes (`TestFontsAreTheOnesCompiledIntoResvg` checks this). They
must also match `../fontmeasure/testdata/*.ttf` (`TestFontsMatchFontmeasureTestFonts`).

- **resvg:** `resvg.wasm` comes from
  [go-resvg](https://github.com/boolean-maybe/go-resvg) v0.0.2 (resvg 0.44). Rebuild it from
  the Rust crate in that repository for the `wasm32-wasip1` target. The Go side depends only
  on its exports (`wasm_alloc`, `wasm_free`, `render`) and the 20-byte result header
  described in `internal/resvg/resvg.go`.
- **Fonts:** change `fonts/`, `resvg.wasm` and `../fontmeasure/testdata` in the same change.
- Run `go test ./...` here and look at the output with `MERMAID_TEST_OUT=<dir>`, which
  writes a PNG of each sample diagram and `flowchart.svg`.

### Release order

A change to the fonts alone lands in one pull request (`fonts/`, `resvg.wasm` and
`../fontmeasure/testdata` together) and then a tag of this module, since this module passes
its own font bytes to `fontmeasure`. This module requires a tagged pele-go, though, so a
change to `fontmeasure`'s code, or to anything else in pele-go this module relies on, takes
three steps, and one pull request can't do all of them:

1. Tag the root module (`vX.Y.Z`).
2. Bump `github.com/Broderick-Westrope/pele-go` in this module's `go.mod` to that tag.
3. Tag this module (`raster/vX.Y.Z`).

Before step 1, open the raster (ubuntu-latest, minimum Go) job's summary on the commit you
tag and check it says raster passes against pele-go at that commit. The step only warns,
so the check itself stays green either way. A tag is permanent once the Go module proxy
has fetched it. To withdraw a bad release, tag a new one whose `go.mod` has a `retract`
directive for it.

## Licence

MIT, see [LICENSE](LICENSE). `internal/resvg/resvg.wasm` is resvg and go-resvg, Apache-2.0,
see [internal/resvg/LICENSE](internal/resvg/LICENSE). The DejaVu fonts are under the
Bitstream Vera licence, see [fonts/LICENSE](fonts/LICENSE). Both are compiled into your
binary, as is pele-go itself, which is MIT licensed with Pele's notice (see its
[LICENSE](https://github.com/Broderick-Westrope/pele-go/blob/main/LICENSE)).
