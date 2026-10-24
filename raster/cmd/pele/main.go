// Command pele renders a Mermaid diagram to PNG or SVG.
//
// Usage:
//
//	pele [flags] [input.mmd]
//
// It reads the diagram from the named file, or from standard input when
// there is none or it is "-". It writes PNG unless -o ends in ".svg" or
// -format says otherwise.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/raster"
)

var themes = map[string]raster.Theme{
	"light": {
		"pele-bg":      "#ffffff",
		"pele-fg":      "#1f1f1f",
		"pele-line":    "#1f1f1f",
		"pele-surface": "#f3f3f3",
		"pele-border":  "#8a8a8a",
		"pele-accent":  "#5b7bd5",
	},
	"dark": {
		"pele-bg":          "#1a1b26",
		"pele-fg":          "#c0caf5",
		"pele-muted":       "#787c99",
		"pele-line":        "#a9b1d6",
		"pele-surface":     "#24283b",
		"pele-surface-alt": "#2f3549",
		"pele-border":      "#565f89",
		"pele-accent":      "#7aa2f7",
	},
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "pele:", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pele", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: pele [flags] [input.mmd]")
		fs.PrintDefaults()
	}
	out := fs.String("o", "-", "output file, or - for standard output")
	format := fs.String("format", "", "png or svg (default: from -o's extension, else png)")
	scale := fs.Float64("scale", 2, "PNG pixels per SVG pixel")
	width := fs.Int("width", 900, "available width in pixels")
	themeName := fs.String("theme", "light", "light or dark")
	transparent := fs.Bool("transparent", false, "leave the PNG background transparent")
	timeout := fs.Duration("timeout", 10*time.Second, "how long rasterizing may take")
	var vars themeVars
	fs.Var(&vars, "var", "override a theme variable, e.g. -var pele-accent=#ff0000 (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return errors.New("at most one input file")
	}

	theme, ok := themes[*themeName]
	if !ok {
		return fmt.Errorf("unknown theme %q, want light or dark", *themeName)
	}
	theme = merge(theme, vars)

	if *format == "" {
		*format = "png"
		if strings.EqualFold(filepath.Ext(*out), ".svg") {
			*format = "svg"
		}
	}
	if *format != "png" && *format != "svg" {
		return fmt.Errorf("unknown format %q, want png or svg", *format)
	}

	src, err := readInput(fs.Arg(0), stdin)
	if err != nil {
		return err
	}

	ctx := context.Background()
	r, err := raster.NewRenderer()
	if err != nil {
		return err
	}
	res, err := r.Render(ctx, src, raster.Options{MaxWidth: *width})
	if err != nil {
		return describe(err)
	}

	var data []byte
	if *format == "svg" {
		svg, err := raster.ResolveVars(res.SVG, theme)
		if err != nil {
			return fmt.Errorf("apply theme: %w", err)
		}
		data = []byte(svg)
	} else {
		data, err = rasterize(ctx, res, theme, *scale, *timeout, !*transparent)
		if err != nil {
			return err
		}
	}

	if *out == "-" {
		_, err = stdout.Write(data)
		return err
	}
	return os.WriteFile(*out, data, 0o644)
}

func rasterize(ctx context.Context, res raster.Result, theme raster.Theme, scale float64, timeout time.Duration, opaque bool) ([]byte, error) {
	rast, err := raster.NewRasterizer(ctx, cacheDir())
	if err != nil {
		return nil, fmt.Errorf("start resvg: %w", err)
	}
	defer rast.Close()
	img, err := rast.Rasterize(res, theme, raster.RasterOptions{Scale: scale, Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("rasterize: %w", err)
	}

	var final image.Image = img
	if opaque {
		bg, err := parseHex(theme["pele-bg"])
		if err != nil {
			return nil, fmt.Errorf("pele-bg: %w", err)
		}
		canvas := image.NewRGBA(img.Bounds())
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
		draw.Draw(canvas, canvas.Bounds(), img, img.Bounds().Min, draw.Over)
		final = canvas
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, final); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// cacheDir keeps compiled resvg between runs, so only the first run pays
// the few hundred milliseconds compiling takes. With no user cache
// directory every run compiles.
func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir = filepath.Join(dir, "pele-go", "resvg")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	return dir
}

func readInput(path string, stdin io.Reader) (string, error) {
	var data []byte
	var err error
	if path == "" || path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return string(data), nil
}

// describe prefixes a diagram error with its code. The message already
// names the line and shows the offending text.
func describe(err error) error {
	var pe *pele.Error
	if !errors.As(err, &pe) {
		return err
	}
	return fmt.Errorf("%s: %s", pe.Code, pe.Message)
}

func merge(base raster.Theme, overrides themeVars) raster.Theme {
	theme := make(raster.Theme, len(base)+len(overrides))
	for k, v := range base {
		theme[k] = v
	}
	for k, v := range overrides {
		theme[k] = v
	}
	return theme
}

func parseHex(s string) (color.RGBA, error) {
	hex, ok := strings.CutPrefix(s, "#")
	if !ok || (len(hex) != 6 && len(hex) != 3) {
		return color.RGBA{}, fmt.Errorf("%q is not a #rgb or #rrggbb colour", s)
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("%q is not a #rgb or #rrggbb colour", s)
	}
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 0xff}, nil
}

// themeVars collects repeated -var name=value flags.
type themeVars map[string]string

func (v *themeVars) String() string { return "" }

func (v *themeVars) Set(s string) error {
	name, value, ok := strings.Cut(s, "=")
	name = strings.TrimPrefix(strings.TrimSpace(name), "--")
	if !ok || name == "" || value == "" {
		return fmt.Errorf("want name=value, got %q", s)
	}
	if *v == nil {
		*v = themeVars{}
	}
	(*v)[name] = value
	return nil
}
