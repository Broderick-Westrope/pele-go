package er

import "github.com/Broderick-Westrope/pele-go/internal/svg"

// markerLength is how far a marker reaches back along its line from the box it touches.
const markerLength = 18

const (
	markerBar        = 5
	markerFoot       = 6
	markerFootLength = 10
	markerRing       = 3.5
)

// erMarker draws a crow's foot cardinality marker over the end of a line that meets a box at
// (x, y) while travelling along the unit vector (dx, dy). Types are Mermaid's lower-cased
// cardinalities.
func erMarker(typ string, x, y, dx, dy float64) string {
	at := func(back, side float64) string {
		return svg.Num(x-float64(dx*back)-float64(dy*side)) + "," + svg.Num(y-float64(dy*back)+float64(dx*side))
	}
	bar := func(back float64) string { return "M" + at(back, -markerBar) + "L" + at(back, markerBar) }
	foot := "M" + at(0, -markerFoot) + "L" + at(markerFootLength, 0) + "L" + at(0, markerFoot)
	ring := func(back float64) string {
		return `<circle class="pele-marker" cx="` + svg.Num(x-float64(dx*back)) + `" cy="` + svg.Num(y-float64(dy*back)) +
			`" r="3.5" fill="var(--_bg)"/>`
	}
	path := func(d string) string { return `<path class="pele-marker" d="` + d + `"/>` }

	switch typ {
	case "only_one":
		return path(bar(6) + bar(10))
	case "zero_or_one":
		return path(bar(6)) + ring(13.5)
	case "one_or_more":
		return path(foot + bar(markerFootLength+3.5))
	case "zero_or_more":
		return path(foot) + ring(markerFootLength+markerRing+0.5)
	case "md_parent":
		return `<path class="pele-marker" d="M` + at(0, 0) + "L" + at(6, 4) + "L" + at(12, 0) + "L" + at(6, -4) + `Z" fill="var(--_bg)"/>`
	}
	return ""
}
