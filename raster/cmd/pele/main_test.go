package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const flowchart = "flowchart LR\n  A[Mermaid] --> B[Pixels]"

func TestRun(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	t.Run("png to stdout over the theme background", func(t *testing.T) {
		var out bytes.Buffer
		err := run([]string{"-theme", "dark", "-scale", "1"}, strings.NewReader(flowchart), &out, &bytes.Buffer{})
		require.NoError(t, err)
		img, err := png.Decode(&out)
		require.NoError(t, err)
		r, g, b, a := img.At(0, 0).RGBA()
		require.Equal(t, [4]uint32{0x1a1a, 0x1b1b, 0x2626, 0xffff}, [4]uint32{r, g, b, a})
	})

	t.Run("transparent png", func(t *testing.T) {
		var out bytes.Buffer
		err := run([]string{"-transparent"}, strings.NewReader(flowchart), &out, &bytes.Buffer{})
		require.NoError(t, err)
		img, err := png.Decode(&out)
		require.NoError(t, err)
		_, _, _, a := img.At(0, 0).RGBA()
		require.Zero(t, a)
	})

	t.Run("svg chosen by extension with theme resolved", func(t *testing.T) {
		dir := t.TempDir()
		in := filepath.Join(dir, "in.mmd")
		require.NoError(t, os.WriteFile(in, []byte(flowchart), 0o600))
		outPath := filepath.Join(dir, "out.svg")
		err := run([]string{"-o", outPath, "-var", "pele-accent=#ff0000", in}, nil, &bytes.Buffer{}, &bytes.Buffer{})
		require.NoError(t, err)
		svg, err := os.ReadFile(outPath)
		require.NoError(t, err)
		require.True(t, bytes.HasPrefix(svg, []byte("<svg ")))
		require.NotContains(t, string(svg), "var(--pele-")
	})

	t.Run("syntax error carries its code", func(t *testing.T) {
		err := run(nil, strings.NewReader("flowchart LR\n  A --> B\n  C -->"), &bytes.Buffer{}, &bytes.Buffer{})
		require.ErrorContains(t, err, "syntax: Parse error on line")
	})

	t.Run("unknown theme", func(t *testing.T) {
		err := run([]string{"-theme", "neon"}, strings.NewReader(flowchart), &bytes.Buffer{}, &bytes.Buffer{})
		require.ErrorContains(t, err, `unknown theme "neon"`)
	})
}
