# pele

Renders a Mermaid diagram to PNG or SVG from the command line. It's a small demo of the
[raster](../../README.md) package: pele-go lays the diagram out, and resvg, running in
WebAssembly, draws it. You don't need cgo, a browser or Node.

## Run

From the `raster` directory:

```sh
go run ./cmd/pele -o diagram.png diagram.mmd
```

Or install it:

```sh
go install ./cmd/pele
```

## Examples

```sh
# Dark PNG at 3x
pele -theme dark -scale 3 -o diagram.png diagram.mmd

# SVG with the theme's colours filled in, read from stdin
echo 'flowchart LR
  A --> B' | pele -o diagram.svg

# Transparent PNG with a custom accent, written to stdout
pele -transparent -var pele-accent=#e45756 diagram.mmd > diagram.png
```

## Gallery

pele drew these from the sources in [`examples`](examples). To redraw them, run these from
that directory:

```sh
pele -o pipeline.png pipeline.mmd
```

![A flowchart from Mermaid source through pele-go and resvg to PNG](examples/pipeline.png)

```sh
pele -theme dark -o sequence.png sequence.mmd
```

![A dark sequence diagram of the CLI calling pele-go and resvg](examples/sequence.png)

```sh
pele -var pele-bg=#fdf6e3 -var pele-fg=#3b3227 -var pele-line=#3b3227 \
  -var pele-surface=#f4e9cf -var pele-border=#b58900 -o states.png states.mmd
```

<img src="examples/states.png" width="250" alt="A state diagram in a custom warm theme">

## Flags

| Flag | Default | |
|---|---|---|
| `-o` | `-` | Output file; `-` is standard output |
| `-format` | from `-o` | `png` or `svg`; `.svg` outputs default to SVG, everything else to PNG |
| `-theme` | `light` | `light` or `dark` |
| `-var name=value` | | Override one theme variable, e.g. `pele-bg=#000000`; repeatable |
| `-scale` | `2` | PNG pixels per SVG pixel |
| `-width` | `900` | Width available to the layout, in pixels |
| `-transparent` | `false` | Don't paint `pele-bg` behind the PNG |
| `-timeout` | `10s` | How long rasterizing may take |

The first run compiles resvg, which takes a few hundred milliseconds. Later runs load the
compiled code from the user cache directory (`pele-go/resvg`).
