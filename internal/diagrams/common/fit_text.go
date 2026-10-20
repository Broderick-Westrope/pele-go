package common

import (
	"math"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// Fitted is one line of text fitted into a width.
type Fitted struct {
	Text  string
	Size  float64
	Width float64
}

// codePoints is Array.from(s): s cut into code points, a lone surrogate on its own.
func codePoints(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		_, n := jsstr.Decode(s, i)
		out = append(out, s[i:i+n])
		i += n
	}
	return out
}

// FitText fits one line of text into a width: first by shrinking the font down to min,
// then by cutting the text and adding an ellipsis. It reports false when not even one
// character fits.
func FitText(s string, max, size, min float64, measurer text.Measurer, style text.Style) (Fitted, bool) {
	if s == "" || max <= 0 {
		return Fitted{}, false
	}
	width := measurer.Width(s, size, style)
	if width <= max {
		return Fitted{Text: s, Size: size, Width: width}, true
	}

	// Width grows with font size, so one division lands on the right size or just above it.
	fitted := math.Max(min, math.Floor((size*max)/width))
	for fitted >= min {
		width = measurer.Width(s, fitted, style)
		if width <= max {
			return Fitted{Text: s, Size: fitted, Width: width}, true
		}
		fitted--
	}

	chars := codePoints(s)
	cut := func(n int) string { return jsstr.TrimEnd(strings.Join(chars[:n], "")) + "…" }
	low := 0
	high := len(chars) - 1
	for low < high {
		mid := (low + high + 1) >> 1
		if measurer.Width(cut(mid), min, style) <= max {
			low = mid
		} else {
			high = mid - 1
		}
	}
	if low == 0 {
		return Fitted{}, false
	}
	t := cut(low)
	width = measurer.Width(t, min, style)
	if width > max {
		return Fitted{}, false
	}
	return Fitted{Text: t, Size: min, Width: width}, true
}
