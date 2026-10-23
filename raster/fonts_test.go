package raster_test

import (
	"crypto/sha256"
	"fmt"
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

// pele-go's fontmeasure tests measure with its own copies of the fonts, so
// they only vouch for these if the files are identical. The fonts also have
// to match the ones compiled into resvg.wasm; see the README.
func TestFontsMatchFontmeasureTestFonts(t *testing.T) {
	t.Parallel()

	theirs := filepath.Join("..", "fontmeasure", "testdata")
	if _, err := os.Stat(theirs); err != nil {
		t.Skipf("pele-go's fontmeasure is not next to this module (tested from the module cache?): %v", err)
	}
	ours := hashFonts(t, "fonts")
	require.NotEmpty(t, ours)
	require.Equal(t, ours, hashFonts(t, theirs))
}

func hashFonts(t *testing.T, dir string) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.ttf"))
	require.NoError(t, err)
	sums := map[string]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		sums[filepath.Base(f)] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	return sums
}
