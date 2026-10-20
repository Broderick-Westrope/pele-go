package svg

import (
	"math"

	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// Titled is content with a title put above it.
type Titled struct {
	Width, Height float64
	Content       string
}

// WithTitle puts a diagram title above content that was laid out without one. An empty
// title is upstream's undefined.
func WithTitle(title string, measurer text.Measurer, size, pad, width, height float64, content string, icons *text.Icons) Titled {
	label := text.Layout(title, false, measurer, size, 4000, text.Bold, icons)
	if label.Height == 0 {
		return Titled{Width: width, Height: height, Content: content}
	}
	shift := label.Height + 12
	full := max(width, math.Ceil(label.Width+float64(2*pad)))
	return Titled{
		Width:  full,
		Height: math.Ceil(height + shift),
		Content: LabelSVG(label, full/2, pad+float64(label.Height/2), ` class="pele-title" font-weight="var(--_tw)"`, nil) +
			`<g transform="translate(` + Num((full-width)/2) + `,` + Num(shift) + `)">` + content + `</g>`,
	}
}
