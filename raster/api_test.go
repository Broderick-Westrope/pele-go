package raster_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Broderick-Westrope/pele-go/raster"
	"github.com/stretchr/testify/require"
)

// A Rasterizer runs every render under the context it was made with, so
// one whose deadline has already passed times out on its first render,
// however fast the platform is.
func TestRasterizePastItsDeadlineReturnsErrRasterTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	rast, err := raster.NewRasterizer(ctx, "")
	require.NoError(t, err, "making a Rasterizer doesn't check the context")
	t.Cleanup(func() { require.NoError(t, rast.Close(), "Close still frees everything") })

	res := raster.Result{SVG: `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`, Width: 10, Height: 10}
	_, err = rast.Rasterize(res, nil, raster.RasterOptions{Scale: 1})
	require.ErrorIs(t, err, raster.ErrRasterTimeout)
	require.NotErrorIs(t, err, raster.ErrRenderTimeout, "it's the rasterizing timeout, not the layout one")
}

func nestedSubgraphs(depth int) string {
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	for i := range depth {
		fmt.Fprintf(&b, "subgraph s%d\n", i)
	}
	b.WriteString("a --> b\n")
	b.WriteString(strings.Repeat("end\n", depth))
	return b.String()
}

func TestRasterizeStopsAtItsTimeoutAndRecovers(t *testing.T) {
	t.Parallel()

	r, err := raster.NewRenderer()
	require.NoError(t, err)
	rast, err := raster.NewRasterizer(t.Context(), "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rast.Close() })
	// Nested clusters paint overlapping rects, so this takes about a
	// second, far past the timeout.
	res, err := r.Render(t.Context(), nestedSubgraphs(10), raster.Options{MaxWidth: 1000})
	require.NoError(t, err)

	start := time.Now()
	_, err = rast.Rasterize(res, nil, raster.RasterOptions{Scale: 2, Timeout: time.Millisecond})
	require.ErrorIs(t, err, raster.ErrRasterTimeout)
	require.Less(t, time.Since(start), 500*time.Millisecond)

	_, err = rast.Rasterize(res, nil, raster.RasterOptions{Scale: 0.25})
	require.NoError(t, err, "a zero Timeout uses the default, and the next raster starts fresh")
}

func TestResolveVarsLeavesColorMix(t *testing.T) {
	t.Parallel()

	const svg = `<svg style="--a:#ff0000"><rect fill="color-mix(in srgb,var(--a) 50%,#0000ff)"/></svg>`
	got, err := raster.ResolveVars(svg, nil)
	require.NoError(t, err)
	require.Contains(t, got, `fill="color-mix(in srgb,#ff0000 50%,#0000ff)"`)
}

// The MaxPixels doc promises room for at least 16M pixels under resvg's
// memory cap.
func TestRasterize16MegapixelsFitsResvgMemory(t *testing.T) {
	t.Parallel()

	rast, err := raster.NewRasterizer(t.Context(), "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rast.Close() })
	res := raster.Result{SVG: `<svg xmlns="http://www.w3.org/2000/svg" width="4096" height="4096"><rect width="4096" height="4096" fill="#f00"/></svg>`, Width: 4096, Height: 4096}

	img, err := rast.Rasterize(res, nil, raster.RasterOptions{Scale: 1, MaxPixels: 16 << 20})
	require.NoError(t, err)
	require.Equal(t, 4096, img.Bounds().Dx())
	require.Equal(t, 4096, img.Bounds().Dy())
}
