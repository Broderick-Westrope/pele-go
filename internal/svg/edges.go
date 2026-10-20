package svg

import (
	"math"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/layout"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// Arrow is the length of an arrowhead.
const Arrow = 8

// EdgePath is an edge's path, with where and which way it leaves its start and arrives at
// its end.
type EdgePath struct {
	D                string
	SX, SY, SDX, SDY float64
	EX, EY, EDX, EDY float64
}

// Marker types are Mermaid's: arrow_point, arrow_circle, arrow_cross; anything else draws
// nothing.

// corner is the most a corner is rounded by in a stepped route.
const corner = 12

// handle is how far the handles of a wide turn reach along its two legs. A little more than
// a circle's, which makes the turn fuller.
const handle = 0.62

// Lead is how far an edge runs straight before a marker at its end, where it has the room.
const Lead = 6

// SideReach records how far from each side of each node the edges that meet it start to
// turn: as far as the largest marker there needs, for every edge on the side, so that none
// turns sooner than its neighbour and crosses it. The zero value is empty.
type SideReach struct {
	reach map[int]float64
}

// Add takes a node, the side of it, and the marker of an end there.
func (r *SideReach) Add(node, side int, typ string) {
	trim := MarkerTrim(typ)
	if side >= 0 && trim > 0 && trim+Lead > r.reach[node*4+side] {
		if r.reach == nil {
			r.reach = map[int]float64{}
		}
		r.reach[node*4+side] = trim + Lead
	}
}

// Of returns the reach of a side of a node.
func (r *SideReach) Of(node, side int) float64 {
	if side < 0 {
		return 0
	}
	return r.reach[node*4+side]
}

// signOr1 is Math.sign(v) || 1.
func signOr1(v float64) float64 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 1
}

// sign is Math.sign.
func sign(v float64) float64 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return v
}

// DefaultReach is how far from its node an edge starts to turn when nothing beside it needs
// more: far enough to clear its own marker.
func DefaultReach(trim float64) float64 {
	if trim > 0 {
		return trim + Lead
	}
	return 0
}

// RoutePath builds the path for a route of x, y, axis triples, trimming both ends to leave
// room for markers. Between two points that are not in line, a route is one S that leaves
// and arrives along the flow, or a step in a stepped route. Where the route itself turns a
// corner, a stepped route rounds it a little, and any other but a linear one turns as widely
// as it can: the turn takes the whole of a leg that ends at a node and half of a leg it
// shares with the next turn. startReach and endReach say how far from its nodes the edge
// starts to turn, where the other edges on the same side of a node need it to; upstream's
// default is DefaultReach of the trim. curve "" is upstream's undefined.
func RoutePath(route []float64, curve string, startTrim, endTrim, startReach, endReach float64) EdgePath {
	linear := curve == "linear"
	stepped := curve == "step" || curve == "stepBefore" || curve == "stepAfter"
	var xs, ys, axes []float64
	near := func(ax, ay, bx, by float64) bool { return math.Abs(ax-bx) < 0.01 && math.Abs(ay-by) < 0.01 }
	for i := 0; i < len(route); i += 3 {
		x, y, axis := route[i], route[i+1], route[i+2]
		last := len(xs) - 1
		// A stepped route turns at right angles where a smooth one would curve.
		if stepped && last >= 0 && math.Abs(x-xs[last]) >= 0.01 && math.Abs(y-ys[last]) >= 0.01 {
			px, py := xs[last], ys[last]
			var my, mx float64
			switch curve {
			case "stepBefore":
				my, mx = py, px
			case "stepAfter":
				my, mx = y, x
			default:
				my, mx = (py+y)/2, (px+x)/2
			}
			turns := [4]float64{mx, py, mx, y}
			if axis == 0 {
				turns = [4]float64{px, my, x, my}
			}
			for k := 0; k < 4; k += 2 {
				if near(turns[k], turns[k+1], px, py) || near(turns[k], turns[k+1], x, y) {
					continue
				}
				xs = append(xs, turns[k])
				ys = append(ys, turns[k+1])
				axes = append(axes, axis)
			}
		}
		xs = append(xs, x)
		ys = append(ys, y)
		axes = append(axes, axis)
	}
	last := len(xs) - 1
	aligned := func(i int) bool { return math.Abs(xs[i]-xs[i-1]) < 0.01 || math.Abs(ys[i]-ys[i-1]) < 0.01 }
	straight := func(i int) bool { return linear || aligned(i) }
	// The way the route heads from point i to point j: along the flow where an S joins them.
	unit := func(i, j int, axis float64) (float64, float64) {
		if !straight(max(i, j)) {
			if axis == 0 {
				return 0, signOr1(ys[j] - ys[i])
			}
			return signOr1(xs[j] - xs[i]), 0
		}
		dx := xs[j] - xs[i]
		dy := ys[j] - ys[i]
		length := jsmath.Hypot(dx, dy)
		if length == 0 || math.IsNaN(length) {
			length = 1
		}
		return dx / length, dy / length
	}
	sdx, sdy := unit(0, 1, axes[1])
	edx, edy := unit(last-1, last, axes[last])
	sx, sy := xs[0], ys[0]
	ex, ey := xs[last], ys[last]
	xs[0] += float64(sdx * startTrim)
	ys[0] += float64(sdy * startTrim)
	xs[last] -= float64(edx * endTrim)
	ys[last] -= float64(edy * endTrim)
	length := func(i int) float64 { return math.Abs(xs[i]-xs[i-1]) + math.Abs(ys[i]-ys[i-1]) }
	// Whether the route turns a corner at point i, from running one way to running the other.
	turns := func(i int) bool {
		return !linear && i > 0 && i < last && aligned(i) && aligned(i+1) && length(i) >= 0.01 && length(i+1) >= 0.01 &&
			(math.Abs(xs[i]-xs[i-1]) < 0.01) != (math.Abs(xs[i+1]-xs[i]) < 0.01)
	}
	// An edge that curves into a marker runs straight for a little way first, where it has
	// the room, so that the marker does not sit on the bend.
	along := func(i int, ux, uy float64) float64 {
		return float64(math.Abs(xs[i]-xs[i-1])*math.Abs(ux)) + float64(math.Abs(ys[i]-ys[i-1])*math.Abs(uy))
	}
	// Of the way from the end of a run to where it turns, the part that makes up for a larger
	// marker beside this one is kept where it can be, and the rest is at most a quarter of
	// what is left, so that ends on one side of a node turn equally far from it.
	lead := func(reach, trim, run float64) float64 {
		room := min(max(0, reach-Lead-trim), run/4)
		return room + min(reach-trim-room, (run-room)/4)
	}
	arrives, leaves := 0.0, 0.0
	if endReach > endTrim && (!straight(last) || turns(last-1)) {
		arrives = lead(endReach, endTrim, along(last, edx, edy))
	}
	if startReach > startTrim && (!straight(1) || turns(1)) {
		leaves = lead(startReach, startTrim, along(1, sdx, sdy))
	}
	if arrives >= 1 {
		xs = insertAt(xs, last, xs[last]-float64(edx*arrives))
		ys = insertAt(ys, last, ys[last]-float64(edy*arrives))
		axes = insertAt(axes, last, axes[last])
		last++
	}
	if leaves >= 1 {
		xs = insertAt(xs, 1, xs[0]+float64(sdx*leaves))
		ys = insertAt(ys, 1, ys[0]+float64(sdy*leaves))
		axes = insertAt(axes, 1, axes[1])
		last++
	}

	// 1 - handle in float64, as JS computes it, not as an exact constant.
	h := handle
	keep := 1 - h
	d := "M" + Num(xs[0]) + "," + Num(ys[0])
	atX, atY := xs[0], ys[0]
	lineTo := func(x, y float64) {
		if near(x, y, atX, atY) {
			return
		}
		d += "L" + Num(x) + "," + Num(y)
		atX, atY = x, y
	}
	for i := 1; i <= last; i++ {
		x, y := xs[i], ys[i]
		if !straight(i) {
			down := axes[i] == 0
			var run, across float64
			if down {
				run, across = y-atY, x-atX
			} else {
				run, across = x-atX, y-atY
			}
			k := float64(math.Abs(run) * layout.Ease(across, run))
			var ux, uy float64
			if down {
				uy = float64(sign(run) * k)
			} else {
				ux = float64(sign(run) * k)
			}
			d += "C" + Num(atX+ux) + "," + Num(atY+uy) + " " + Num(x-ux) + "," + Num(y-uy) + " " + Num(x) + "," + Num(y)
			atX, atY = x, y
			continue
		}
		if !turns(i) {
			// A point part of the way along one straight run adds nothing to the path.
			if i < last && aligned(i) && aligned(i+1) && float64((x-xs[i-1])*(xs[i+1]-x))+float64((y-ys[i-1])*(ys[i+1]-y)) > 0 {
				continue
			}
			lineTo(x, y)
			continue
		}
		// How much of the leg before the turn and of the leg after it the turn takes: all of
		// one that ends at a node, short of the straight run into a marker, and half of one
		// it shares.
		before := length(i)
		if turns(i - 1) {
			before = length(i) / 2
		}
		after := length(i + 1)
		if turns(i + 1) {
			after = length(i+1) / 2
		}
		if stepped {
			before = min(corner, before, after)
			after = before
		}
		ux, uy := unit(i-1, i, axes[i])
		vx, vy := unit(i, i+1, axes[i+1])
		lineTo(x-float64(ux*before), y-float64(uy*before))
		atX = x + float64(vx*after)
		atY = y + float64(vy*after)
		if stepped {
			d += "Q" + Num(x) + "," + Num(y) + " " + Num(atX) + "," + Num(atY)
		} else {
			d += "C" + Num(x-float64(float64(ux*before)*keep)) + "," + Num(y-float64(float64(uy*before)*keep)) +
				" " + Num(x+float64(float64(vx*after)*keep)) + "," + Num(y+float64(float64(vy*after)*keep)) +
				" " + Num(atX) + "," + Num(atY)
		}
	}
	return EdgePath{D: d, SX: sx, SY: sy, SDX: -sdx, SDY: -sdy, EX: ex, EY: ey, EDX: edx, EDY: edy}
}

func insertAt(list []float64, at int, v float64) []float64 {
	list = append(list, 0)
	copy(list[at+1:], list[at:])
	list[at] = v
	return list
}

// MarkerTrim is how much of an edge a marker of the type takes.
func MarkerTrim(typ string) float64 {
	switch typ {
	case "arrow_point":
		return Arrow - 1
	case "arrow_circle":
		return Arrow
	case "arrow_cross":
		return 4
	}
	return 0
}

// Marker draws a marker whose tip is at (x, y), pointing along (dx, dy).
func Marker(typ string, x, y, dx, dy float64, color string) string {
	nx := -dy
	ny := dx
	switch typ {
	case "arrow_point":
		bx := x - float64(dx*Arrow)
		by := y - float64(dy*Arrow)
		const half = Arrow * 0.42
		return `<path class="pele-marker" d="M` + Num(x) + "," + Num(y) +
			"L" + Num(bx+float64(nx*half)) + "," + Num(by+float64(ny*half)) +
			"L" + Num(bx-float64(nx*half)) + "," + Num(by-float64(ny*half)) +
			`Z" fill="` + color + `" stroke="none"/>`
	case "arrow_circle":
		return `<circle class="pele-marker" cx="` + Num(x-float64(dx*4)) + `" cy="` + Num(y-float64(dy*4)) +
			`" r="3.5" fill="` + color + `" stroke="none"/>`
	case "arrow_cross":
		cx := x - float64(dx*4)
		cy := y - float64(dy*4)
		const a = 3.5
		ux := float64((dx + nx) * a)
		uy := float64((dy + ny) * a)
		vx := float64((dx - nx) * a)
		vy := float64((dy - ny) * a)
		return `<path class="pele-marker" d="M` + Num(cx-ux) + "," + Num(cy-uy) + "L" + Num(cx+ux) + "," + Num(cy+uy) +
			"M" + Num(cx-vx) + "," + Num(cy-vy) + "L" + Num(cx+vx) + "," + Num(cy+vy) + `"/>`
	}
	return ""
}

// EdgeLabelSVG draws an edge label on a patch of the background, so the line does not run
// through the text.
func EdgeLabelSVG(id string, label text.Label, x, y float64, attrs string, icons *text.Icons) string {
	w := label.Width + 8
	h := label.Height
	return `<g class="pele-edge-label" data-id="` + EscText(id) + `">` +
		`<rect x="` + Num(x-float64(w/2)) + `" y="` + Num(y-float64(h/2)) + `" width="` + Num(w) + `" height="` + Num(h) +
		`" rx="3" fill="var(--_bg)"/>` +
		LabelSVG(label, x, y, attrs, icons) +
		`</g>`
}

// LoopPath draws a self-loop beside a node of drawn size w by h centred on (cx, cy): out of
// the side that lies across the flow and back in. k counts the loops already on the node, so
// each one reaches a little further. It also places the edge's label, labelW by labelH,
// beyond the loop, returning its centre.
func LoopPath(cx, cy, w, h, k float64, sideways bool, base, trim, labelW, labelH float64) (path EdgePath, labelX, labelY float64) {
	half := h / 2
	if sideways {
		half = w / 2
	}
	spread := min(half-4, 8+float64(k*6))
	reach := base + float64(k*8)
	length := jsmath.Hypot(reach, spread)
	if length == 0 || math.IsNaN(length) {
		length = 1
	}
	// Local frame: out points away from the node, along runs along its side.
	at := func(out, along float64) string {
		if sideways {
			return Num(cx+along) + "," + Num(cy+float64(h/2)+out)
		}
		return Num(cx+float64(w/2)+out) + "," + Num(cy+along)
	}
	tx := float64((reach / length) * trim)
	ty := float64((spread / length) * trim)
	if sideways {
		labelX = cx
		labelY = cy + float64(h/2) + reach + 4 + float64(labelH/2)
	} else {
		labelX = cx + float64(w/2) + reach + 4 + float64(labelW/2)
		labelY = cy
	}
	path = EdgePath{
		D:   "M" + at(0, -spread) + "C" + at(reach, float64(-spread*2)) + " " + at(reach, float64(spread*2)) + " " + at(tx, spread+ty),
		SX:  cx + float64(w/2),
		SY:  cy - spread,
		SDX: -1,
		EX:  cx + float64(w/2),
		EY:  cy + spread,
		EDX: -reach / length,
		EDY: -spread / length,
	}
	if sideways {
		path.SX, path.SY, path.SDX, path.SDY = cx-spread, cy+float64(h/2), 0, -1
		path.EX, path.EY, path.EDX, path.EDY = cx+spread, cy+float64(h/2), -spread/length, -reach/length
	}
	return path, labelX, labelY
}
