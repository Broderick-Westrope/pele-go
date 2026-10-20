package svg

import (
	"math"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// Box is what the drawing code reads of a node the compound layout placed (upstream's CNode
// from layout/compound.ts): its center, its size, and whether it is a group.
type Box struct {
	X, Y, W, H float64
	IsGroup    bool
}

// Crossings holds the x of every route point, by its rounded y: the edges that cross a
// cluster's top border. Only looked up by key, so a Go map serves for upstream's Map.
type Crossings map[float64][]float64

// MarkCrossings adds the points of a route of x, y, axis triples.
func MarkCrossings(crossings Crossings, route []float64) {
	for k := 0; k < len(route); k += 3 {
		key := jsmath.Round(route[k+1])
		crossings[key] = append(crossings[key], route[k])
	}
}

// ClusterTitleX picks where a cluster title sits along the top edge so that edges entering
// there do not cross it.
func ClusterTitleX(c Box, width float64, crossings Crossings, pad float64) float64 {
	if width == 0 {
		return c.X
	}
	top := c.Y - float64(c.H/2)
	left := c.X - float64(c.W/2)
	half := float64(width / 2)
	candidates := [3]float64{left + pad - 6 + half, c.X, left + c.W - pad + 6 - half}
	xs, ok := crossings[jsmath.Round(top)]
	if !ok {
		return candidates[0]
	}
	best := candidates[0]
	bestGap := -1.0
	for _, x := range candidates {
		gap := math.Inf(1)
		for _, cx := range xs {
			if cx > left && cx < left+c.W {
				gap = min(gap, math.Abs(cx-x)-half)
			}
		}
		if gap >= 6 {
			return x
		}
		if gap > bestGap+0.5 {
			bestGap = gap
			best = x
		}
	}
	return best
}

// TitleCrossed reports whether an edge that enters at the top runs through a title placed
// at x.
func TitleCrossed(c Box, width, x float64, crossings Crossings) bool {
	if !(width > 0) {
		return false
	}
	xs, ok := crossings[jsmath.Round(c.Y-float64(c.H/2))]
	if !ok {
		return false
	}
	left := c.X - float64(c.W/2)
	for _, cx := range xs {
		if cx > left && cx < left+c.W && math.Abs(cx-x) < float64(width/2)+4 {
			return true
		}
	}
	return false
}

// StruckTitle draws a title that an edge runs through, to be drawn over the edges: the text,
// on an outline in the color of the cluster behind it, so that only the line is seen to stop
// short of the letters. parent and index find how many clusters deep the title is, each of
// which tints the background.
func StruckTitle(label text.Label, x, y float64, attrs, id string, parent func(int) int, index int, icons *text.Icons) string {
	depth := 1
	for p := parent(index); p >= 0 && depth < 8; p = parent(p) {
		depth++
	}
	// Each cluster is half its fill over what is behind it. 0.5 ** depth is exact.
	fill := `color-mix(in srgb,var(--_a) ` + Num(100*(1-math.Ldexp(1, -depth))) + `%,var(--_bg))`
	return `<g class="pele-cluster-title" data-id="` + id + `">` +
		LabelSVG(label, x, y, ` fill="none" stroke="`+fill+`" stroke-width="5" stroke-linejoin="round"`, icons) +
		LabelSVG(label, x, y, attrs, icons) +
		`</g>`
}
