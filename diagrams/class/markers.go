package class

import "github.com/Broderick-Westrope/pele-go/internal/svg"

const (
	triangle = 12
	diamond  = 16
	half     = 6
	ring     = 5
)

// classMarkerTrim is how far the line stops short of the node so that it meets the back of
// the marker.
func classMarkerTrim(typ endMarker) float64 {
	switch typ {
	case markerExtension:
		return triangle
	case markerComposition, markerAggregation:
		return diamond
	case markerLollipop:
		return 2 * ring
	case markerDependency:
		return svg.MarkerTrim("arrow_point")
	}
	return 0
}

// classMarker draws an end marker whose tip is at (x, y), pointing along (dx, dy).
func classMarker(typ endMarker, x, y, dx, dy float64, color string) string {
	nx := -dy
	ny := dx
	num := svg.Num
	switch typ {
	case markerExtension:
		bx := x - float64(dx*triangle)
		by := y - float64(dy*triangle)
		return `<path class="pele-marker" d="M` + num(x) + `,` + num(y) +
			`L` + num(bx+float64(nx*half)) + `,` + num(by+float64(ny*half)) +
			`L` + num(bx-float64(nx*half)) + `,` + num(by-float64(ny*half)) + `Z" stroke-linejoin="round"/>`
	case markerComposition, markerAggregation:
		mx := x - float64(float64(dx*diamond)/2)
		my := y - float64(float64(dy*diamond)/2)
		const h = half - 1
		fill := ""
		if typ == markerComposition {
			fill = ` fill="` + color + `"`
		}
		return `<path class="pele-marker" d="M` + num(x) + `,` + num(y) +
			`L` + num(mx+float64(nx*h)) + `,` + num(my+float64(ny*h)) +
			`L` + num(x-float64(dx*diamond)) + `,` + num(y-float64(dy*diamond)) +
			`L` + num(mx-float64(nx*h)) + `,` + num(my-float64(ny*h)) + `Z" stroke-linejoin="round"` + fill + `/>`
	case markerLollipop:
		return `<circle class="pele-marker" cx="` + num(x-float64(dx*ring)) + `" cy="` + num(y-float64(dy*ring)) + `" r="5"/>`
	case markerDependency:
		return svg.Marker("arrow_point", x, y, dx, dy, color)
	}
	return ""
}
