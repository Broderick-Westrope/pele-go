package raster

import _ "embed"

// These are the same DejaVu Sans subsets baked into resvg/resvg.wasm, so
// Pele lays labels out with the widths resvg will actually draw.
var (
	//go:embed fonts/DejaVuSans.ttf
	fontRegular []byte
	//go:embed fonts/DejaVuSans-Bold.ttf
	fontBold []byte
	//go:embed fonts/DejaVuSans-Oblique.ttf
	fontItalic []byte
	//go:embed fonts/DejaVuSans-BoldOblique.ttf
	fontBoldItalic []byte
)

// FontFace is one embedded DejaVu Sans face with its CSS font-weight and
// font-style, for embedding in pages that show the SVG, so a browser draws
// labels in the font they were measured in.
type FontFace struct {
	// Data is the TrueType file. It aliases the embedded bytes, so callers
	// must not modify it.
	Data          []byte
	Weight, Style string
}

// FontFaces returns the faces the Renderer measures labels with and resvg
// draws them in.
func FontFaces() []FontFace {
	return []FontFace{
		{fontRegular, "normal", "normal"},
		{fontBold, "bold", "normal"},
		{fontItalic, "normal", "italic"},
		{fontBoldItalic, "bold", "italic"},
	}
}
