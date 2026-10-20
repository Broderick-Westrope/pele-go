package common

import (
	"github.com/Broderick-Westrope/pele-go/internal/svg"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// IconSVG is a square for a named icon, in the form labels use. The host's resolver supplies
// the drawing, if any.
func IconSVG(name string, x, y, side float64, icons *text.Icons) string {
	return `<svg class="pele-icon" data-icon="` + svg.EscText(name) + `" x="` + svg.Num(x) + `" y="` + svg.Num(y) + `" width="` + svg.Num(side) +
		`" height="` + svg.Num(side) +
		`" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
		icons.Resolve(name) + `</svg>`
}
