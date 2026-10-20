# pele-go

A Go port of [Pele](https://github.com/obsidianmd/pele), the Mermaid-to-SVG renderer. It
produces the same output as Pele 1.7.0 byte for byte (SVG, width, height, links, and error
code, message and position) for the same source, options and text measurer. All 33 of
Pele's diagram types are ported.

The root package is pure Go with no dependencies outside the standard library and no
JavaScript runtime, so it builds with `CGO_ENABLED=0`.

## Install

```sh
go get github.com/Broderick-Westrope/pele-go
```

Go 1.26 or later.

## Use

```go
import (
	pele "github.com/Broderick-Westrope/pele-go"
	_ "github.com/Broderick-Westrope/pele-go/all" // registers every diagram type
)

res, err := pele.Render(ctx, "flowchart LR\n  A[Mermaid text] --> B[SVG]", pele.Options{})
if err != nil {
	return err
}
fmt.Println(res.SVG, res.Width, res.Height)
```

`Result` also carries `Links` (elements that link somewhere), `Type` (`flowchart`) and
`MermaidType` (`flowchart-v2`). Theme the SVG with the same CSS variables as upstream
(`--pele-fg`, `--pele-font` and so on, see [Theming](https://pele.run/theming)).

To keep binaries small, import single types instead of `all` and register them yourself:

```go
import "github.com/Broderick-Westrope/pele-go/diagrams/flowchart"

func init() { pele.Register(flowchart.Diagram) }
```

`Register` must run before the first `Render`, normally from `init`; it panics afterwards.
`DetectType` and `Supports` tell you a source's type without rendering it, and don't count
as a render for `Register`.

### Options

`Options` mirrors upstream's `RenderOptions`. The zero value renders exactly as upstream
does with no options. Fields where JS tells "unset" apart from a zero value are pointers,
and `pele.Ptr` makes one inline:

```go
config, err := pele.ParseConfig(`{"flowchart": {"curve": "linear"}}`)
if err != nil {
	return err
}
res, err := pele.Render(ctx, src, pele.Options{
	FontSize:   pele.Ptr(14.0),
	Responsive: pele.Ptr(false),
	MaxWidth:   pele.Ptr(480.0),
	LinkRel:    pele.Ptr(""), // omit rel; nil means "noopener"
	Config:     config,
	Now:        time.Now(), // what gantt charts treat as today
})
```

`ParseConfig` reads Mermaid configuration from JSON and keeps JavaScript's key order, which
upstream's output can depend on. Front matter and directives in the diagram win over it.
The defaults for every field are in the `Options` doc comment.

Gantt charts read dates in `Options.Location` (default `time.Local`), standing in for the
time zone of the JS process. Set it and `Now` if you need output that doesn't depend on the
machine.

### Measurers

Layout depends on how wide text is, so output is only byte-identical to upstream's when both
use the same measurer. There are two built in:

| Measurer | Use it when |
|---|---|
| `pele.DefaultMeasurer()` (what a nil `Options.Measurer` uses) | You want upstream's output with no options. It's upstream's Arial advance table, ported exactly. |
| `fontmeasure.New(regular, bold, italic, boldItalic)` | You know which fonts will draw the SVG (for example you rasterise it) and want labels sized to them. |

```go
import "github.com/Broderick-Westrope/pele-go/fontmeasure"

m, err := fontmeasure.New(regular, bold, italic, boldItalic) // TTF/OTF bytes you supply
res, err := pele.Render(ctx, src, pele.Options{Measurer: m})
```

`fontmeasure` sums glyph advances and kerning (the GPOS `kern` feature, else the legacy
`kern` table; no ligatures or shaping) at 1024 ppem, measures a missing glyph as
`n`, and takes `fontmeasure.WithMono(...)` for code spans. It embeds no fonts, and it's the
only package that pulls in `golang.org/x/image`. You can also implement `pele.TextMeasurer`
yourself; it must be safe for concurrent use and must not panic (as must `Options.Icons`).

### Errors

Failures are `*pele.Error`. Read one with `errors.As`:

```go
var pe *pele.Error
if errors.As(err, &pe) {
	log.Printf("%s at %d:%d: %s\n%s", pe.Code, pe.Line, pe.Column, pe.Message, pe.Snippet)
}
```

| Code | Meaning |
|---|---|
| `CodeUnsupportedDiagram` | No type detected, or the type isn't registered |
| `CodeSyntax` | The source doesn't parse |
| `CodeSemantic` | It parses but asks for something that doesn't exist |
| `CodeLimit` | Past `Limit`, `OutputLimit` or `MaxEdges`, or where upstream's JS throws a `RangeError` ("Diagram is nested too deeply.") |
| `CodeCanceled` | The context was done (Go only, see below) |
| `CodeInternal` | A panic during the render: a bug, or your `TextMeasurer` or `Icons` panicked (Go only). It wraps a `*pele.PanicError` with the value and stack |

Messages, lines and columns are upstream's. Columns count UTF-16 code units, as upstream's
do.

### Cancellation, concurrency and strings

- `Render` checks its context between phases and inside layout loops. A cancelled render
  returns `CodeCanceled` wrapping `ctx.Err()`, so `errors.Is(err, context.DeadlineExceeded)`
  works, and leaves no goroutine behind. Use a deadline for untrusted input.
- `Render` is safe for concurrent use. There's no global cache and no mutable package state
  beyond the registry, which is sealed by the first render.
- Every string in `Result` and `Error` is valid UTF-8. Where upstream would emit a lone
  surrogate (say, from a `\ud800` escape in YAML config), you get U+FFFD, as encoding the JS
  string with `TextEncoder` would.

## How parity is checked

Byte-identical is checked by string comparison against upstream itself, not by eye:

- **goja:** `internal/oracle` runs the npm `pele@1.7.0` bundle (checksum pinned) in
  [goja](https://github.com/dop251/goja), with the same Go measurer on both sides.
- **V8:** the same bundle under Node, when `node` is on `PATH`. Where goja and V8 disagree
  (lone surrogates, numbers past 2^64), V8 wins.
- **Upstream's corpus:** every source in Pele's `tests/corpus`, with several option sets and
  measurers, and gantt charts in both UTC and `Australia/Sydney` (`TestCorpusParity`).
- **Upstream's test suite:** every public `parse`/`render` call Pele's own Vitest suite
  makes, captured with V8's result and replayed (`TestCapturedParity`).
- **Shared modules:** fixtures of upstream function calls recorded under V8 and replayed
  per package.
- **Fuzzing:** `FuzzParity` (goja) and `FuzzParityV8` (Node) per diagram type:

  ```sh
  PELE_FUZZ_TYPE=flowchart go test -run '^$' -fuzz '^FuzzParity$' -fuzztime 60s .
  ```

goja is test-only; it doesn't end up in your binary.

## Following upstream releases

Each Pele release becomes one change here: port the diff between tags file for file (the Go
files map one-to-one to upstream's `src/`), then bump the pinned oracle and regenerate the
fixtures:

- `internal/oracle`: the npm bundle, its checksum and `Version`; `UpstreamVersion` in
  `pele.go`; `tests/corpus` into `internal/oracle/testdata/corpus`.
- `node scripts/capture-cases.mjs <pele checkout>` re-captures upstream's suite.
- `node scripts/gen-shared-cases.mjs <pele checkout>` and the other `scripts/gen-*.mjs`
  regenerate the shared-module fixtures.

[PORTING.md](PORTING.md) has the rules: the JS semantics that need helpers (number
formatting, `Math.round`, UTF-16 strings, object key order, regexes, fused multiply-add on
arm64) and how to debug a mismatch.

## Differences from upstream

- Browser-only APIs aren't ported: `mount`, zoom, `pele/lazy`, `renderAsync`, the canvas
  measurer and anything that reads `window` or the DOM.
- There's no public `parse` or typed model. Render is the API.
- Public strings are well-formed UTF-8, as described above. Internally the port keeps
  upstream's UTF-16 semantics, including lone surrogates.
- Go additions: a `context.Context` on `Render` with `CodeCanceled`, `CodeInternal` for a
  recovered panic, `Options.Location` for gantt's time zone, and a per-render text-width
  cache in place of upstream's global one (`forgetTextWidths` is gone). None of these
  change output.
- There is no depth limit. Deeply nested sources render as they do in V8, which walks them
  without overflowing (checked to 100,000 levels); the port renders them too (tested at
  20,000). Where upstream's JS throws a `RangeError`, such as
  `String.fromCodePoint` given a fraction, the port returns the same `CodeLimit` error.

## Licence

MIT, see [LICENSE](LICENSE). Pele is MIT licensed by kepano, and its copyright notice is
kept there. The oracle's copy of Pele's bundle is under
[`internal/oracle/testdata/PELE_LICENSE`](internal/oracle/testdata/PELE_LICENSE), and the
DejaVu fonts used to test `fontmeasure` under
[`fontmeasure/testdata/LICENSE`](fontmeasure/testdata/LICENSE). Neither is compiled into
the library.
