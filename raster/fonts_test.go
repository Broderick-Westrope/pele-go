package raster_test

import (
	"bytes"
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

// resvg draws with the fonts compiled into resvg.wasm, so they must be the
// ones the Renderer measures with. The wasm data segments hold each font
// without its long runs of zero bytes, so every stretch between those runs
// must appear in the wasm verbatim.
func TestFontsAreTheOnesCompiledIntoResvg(t *testing.T) {
	t.Parallel()

	wasm, err := os.ReadFile(filepath.Join("internal", "resvg", "resvg.wasm"))
	require.NoError(t, err)
	for _, f := range raster.FontFaces() {
		chunks := nonZeroRuns(f.Data, 8)
		require.NotEmpty(t, chunks)
		for _, c := range chunks {
			require.True(t, bytes.Contains(wasm, c), "%s %s: %d bytes at the font's offset %d are not in resvg.wasm",
				f.Weight, f.Style, len(c), bytes.Index(f.Data, c))
		}
	}
}

// nonZeroRuns splits data at every run of at least minZeros zero bytes,
// dropping the zeros.
func nonZeroRuns(data []byte, minZeros int) [][]byte {
	var chunks [][]byte
	for _, c := range bytes.Split(data, make([]byte, minZeros)) {
		if c = bytes.Trim(c, "\x00"); len(c) > 0 {
			chunks = append(chunks, c)
		}
	}
	return chunks
}
