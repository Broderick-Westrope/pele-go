package raster_test

import (
	"context"
	"fmt"

	"github.com/Broderick-Westrope/pele-go/raster"
)

func Example() {
	ctx := context.Background()
	r, err := raster.NewRenderer()
	if err != nil {
		panic(err)
	}
	res, err := r.Render(ctx, "flowchart LR\n  A --> B", raster.Options{MaxWidth: 900})
	if err != nil {
		panic(err)
	}

	// An empty cache directory compiles resvg every time.
	rast, err := raster.NewRasterizer(ctx, "")
	if err != nil {
		panic(err)
	}
	defer rast.Close()
	img, err := rast.Rasterize(res, raster.Theme{"pele-bg": "#ffffff", "pele-fg": "#000000"},
		raster.RasterOptions{Scale: 2})
	if err != nil {
		panic(err)
	}
	fmt.Println(img.Bounds().Size())
	// Output: (320,128)
}
