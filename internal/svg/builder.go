// Package svg writes SVG markup the way upstream's src/svg does.
package svg

import (
	"math"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

func isControl(c byte) bool {
	return c <= 0x08 || c == 0x0b || c == 0x0c || (c >= 0x0e && c <= 0x1f) || c == 0x7f
}

// Esc escapes text for XML and drops the control characters XML does not allow.
func Esc(s string) string {
	needs := false
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '&' || c == '<' || c == '>' || c == '"' || isControl(c) {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '&':
			b.WriteString("&amp;")
		case c == '<':
			b.WriteString("&lt;")
		case c == '>':
			b.WriteString("&gt;")
		case c == '"':
			b.WriteString("&quot;")
		case isControl(c):
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// EscText escapes text that is only ever read as text: a tooltip, an accessible name, a
// data-id. Entity codes are turned into their characters first.
func EscText(s string) string {
	return Esc(text.DecodeEntities(s))
}

// fraction holds the two decimals of a coordinate, by hundredths.
var fraction = func() [100]string {
	var f [100]string
	for i := 1; i < 100; i++ {
		switch {
		case i%10 == 0:
			f[i] = "." + jsnum.Format(float64(i/10))
		case i < 10:
			f[i] = ".0" + jsnum.Format(float64(i))
		default:
			f[i] = "." + jsnum.Format(float64(i))
		}
	}
	return f
}()

// Num writes a coordinate rounded to two decimals.
func Num(value float64) string {
	n := jsmath.Round(value * 100)
	if n == 0 {
		return "0"
	}
	if n > 0 {
		if n < 1e11 {
			q := math.Floor(n / 100)
			return jsnum.Format(q) + fraction[int(n-q*100)]
		}
	} else if n > -1e11 {
		m := -n
		q := math.Floor(m / 100)
		return "-" + jsnum.Format(q) + fraction[int(m-q*100)]
	}
	return jsnum.Format(n / 100)
}

const (
	boldAttr   = ` font-weight="var(--_w)"`
	italicAttr = ` font-style="italic"`
)

// SpanStyle returns the attributes for a span's emphasis.
func SpanStyle(style text.Style) string {
	s := ""
	if style&text.Bold != 0 {
		s += boldAttr
	}
	if style&text.Italic != 0 {
		s += italicAttr
	}
	return s
}

// SharedStyle returns the emphasis every span of a label has. It is written once, on the
// text element, where a CSS rule for the label reaches it.
func SharedStyle(label text.Label) text.Style {
	shared := text.Bold | text.Italic
	any := false
	for _, line := range label.Lines {
		for _, span := range line {
			if span.Icon != "" || span.Text == "" {
				continue
			}
			shared &= span.Style
			any = true
		}
	}
	if !any {
		return 0
	}
	return shared
}

// TextStyle adds that emphasis to a text element's attributes, unless they give a weight of
// their own.
func TextStyle(attrs string, shared text.Style) string {
	if shared&text.Bold != 0 && !strings.Contains(attrs, ` font-weight="`) {
		attrs += boldAttr
	}
	if shared&text.Italic != 0 && !strings.Contains(attrs, ` font-style="`) {
		attrs += italicAttr
	}
	return attrs
}

// LabelSVG writes a measured label as SVG text centered on (cx, cy).
func LabelSVG(label text.Label, cx, cy float64, attrs string, icons *text.Icons) string {
	lines := label.Lines
	if len(lines) == 0 {
		return ""
	}
	lh := label.LineHeight
	shift := label.Size * 0.35
	y := cy - label.Height/2 + lh/2 + shift

	if len(lines) == 1 && len(lines[0]) == 1 && lines[0][0].Icon == "" && lines[0][0].Style == 0 {
		return `<text` + attrs + ` x="` + Num(cx) + `" y="` + Num(y) + `" text-anchor="middle">` + Esc(lines[0][0].Text) + `</text>`
	}

	shared := SharedStyle(label)
	attrs = TextStyle(attrs, shared)
	var body, extra strings.Builder
	for i, line := range lines {
		hasIcon := false
		for _, span := range line {
			if span.Icon != "" {
				hasIcon = true
			}
		}
		if hasIcon {
			x := cx - label.Widths[i]/2
			for _, span := range line {
				if span.Icon != "" {
					side := label.Size * 1.1
					inner := icons.Resolve(span.Icon)
					extra.WriteString(`<svg class="pele-icon" data-icon="` + EscText(span.Icon) + `" x="` + Num(x) + `" y="` + Num(y-shift-side/2) +
						`" width="` + Num(side) + `" height="` + Num(side) +
						`" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
						inner + `</svg>`)
				} else if span.Text != "" {
					body.WriteString(`<tspan x="` + Num(x) + `" y="` + Num(y) + `" text-anchor="start"` + SpanStyle(span.Style&^shared) + `>` + Esc(span.Text) + `</tspan>`)
				}
				x += span.Width
			}
		} else {
			for j, span := range line {
				pos := ""
				if j == 0 {
					pos = ` x="` + Num(cx) + `" y="` + Num(y) + `"`
				}
				body.WriteString(`<tspan` + pos + SpanStyle(span.Style&^shared) + `>` + Esc(span.Text) + `</tspan>`)
			}
			if len(line) == 0 {
				body.WriteString(`<tspan x="` + Num(cx) + `" y="` + Num(y) + `"> </tspan>`)
			}
		}
		y += lh
	}
	return `<text` + attrs + ` text-anchor="middle" xml:space="preserve">` + body.String() + `</text>` + extra.String()
}
