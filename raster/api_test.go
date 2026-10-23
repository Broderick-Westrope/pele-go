package raster_test

import (
	"context"
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

func TestResolveVarsLeavesColorMix(t *testing.T) {
	t.Parallel()

	const svg = `<svg style="--a:#ff0000"><rect fill="color-mix(in srgb,var(--a) 50%,#0000ff)"/></svg>`
	got, err := raster.ResolveVars(svg, nil)
	require.NoError(t, err)
	require.Contains(t, got, `fill="color-mix(in srgb,#ff0000 50%,#0000ff)"`)
}
