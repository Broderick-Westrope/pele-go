package raster_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Broderick-Westrope/pele-go/raster"
	"github.com/stretchr/testify/require"
)

func TestFontFacesAreTheEmbeddedFonts(t *testing.T) {
	t.Parallel()

	files := map[[2]string]string{
		{"normal", "normal"}: "DejaVuSans.ttf",
		{"bold", "normal"}:   "DejaVuSans-Bold.ttf",
		{"normal", "italic"}: "DejaVuSans-Oblique.ttf",
		{"bold", "italic"}:   "DejaVuSans-BoldOblique.ttf",
	}
	faces := raster.FontFaces()
	require.Len(t, faces, len(files))
	for _, f := range faces {
		file, ok := files[[2]string{f.Weight, f.Style}]
		require.True(t, ok, "unexpected face %s %s", f.Weight, f.Style)
		want, err := os.ReadFile(filepath.Join("fonts", file))
		require.NoError(t, err)
		require.Equal(t, want, f.Data, file)
		delete(files, [2]string{f.Weight, f.Style})
	}
}
