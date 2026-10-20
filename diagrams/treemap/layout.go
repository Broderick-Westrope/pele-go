package treemap

import (
	"math"
	"slices"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
)

type box struct {
	node   *treemapNode
	parent *box
	// series is its position among the top-level nodes of the branch it belongs to, which
	// picks its color.
	series int
	// value is a leaf's own value, or the sum of the leaves under a section.
	value      float64
	children   []*box
	x, y, w, h float64
}

type insets struct {
	gap float64
	// header is the space a section keeps for its header, and side around its children on
	// the other three sides.
	header, side float64
}

func newBox(node *treemapNode, parent *box, series int) *box {
	value := 0.0
	if node.leaf && !math.IsNaN(node.value) && !math.IsInf(node.value, 0) && node.value > 0 {
		value = node.value
	}
	return &box{node: node, parent: parent, series: series, value: value}
}

// squarify lays out boxes in the rectangle as one squarified treemap: rows are added along
// the shorter side, and a row stops growing when another box would make its boxes less
// square.
func squarify(boxes []*box, total, x, y, w, h, gap float64) {
	rest := total
	from := 0
	for from < len(boxes) {
		scale := (w * h) / rest
		side := min(w, h)
		sum := boxes[from].value * scale
		largest := sum
		worst := max((side*side*largest)/(sum*sum), (sum*sum)/(side*side*largest))
		to := from + 1
		for to < len(boxes) {
			smallest := float64(boxes[to].value * scale)
			next := sum + smallest
			ratio := max((side*side*largest)/(next*next), (next*next)/(side*side*smallest))
			if ratio > worst {
				break
			}
			worst = ratio
			sum = next
			to++
		}

		thickness := sum / side
		offset := 0.0
		for i := from; i < to; i++ {
			b := boxes[i]
			length := side - offset
			if i != to-1 {
				length = float64(b.value*scale) / thickness
			}
			bx, by := x, y+offset
			if w < h {
				bx, by = x+offset, y
			}
			half := float64(gap / 2)
			x0 := jsmath.Round(bx + half)
			y0 := jsmath.Round(by + half)
			b.x = x0
			b.y = y0
			across, down := length, thickness
			if w >= h {
				across, down = thickness, length
			}
			b.w = jsmath.Round(bx+across-half) - x0
			b.h = jsmath.Round(by+down-half) - y0
			offset += length
			rest -= b.value
		}
		if w >= h {
			x += thickness
			w -= thickness
		} else {
			y += thickness
			h -= thickness
		}
		from = to
	}
}

// layoutTreemap returns every box with a size, parents before their children.
func layoutTreemap(roots []*treemapNode, width, height float64, in insets) []*box {
	var top []*box
	for i, node := range roots {
		top = append(top, newBox(node, nil, i))
	}
	all := slices.Clone(top)
	for i := 0; i < len(all); i++ {
		parent := all[i]
		for _, node := range parent.node.children {
			child := newBox(node, parent, parent.series)
			parent.children = append(parent.children, child)
			all = append(all, child)
		}
	}
	for i := len(all) - 1; i >= 0; i-- {
		if b := all[i]; b.parent != nil {
			b.parent.value += b.value
		}
	}

	gap, header, side := in.gap, in.header, in.side
	var placed []*box
	place := func(children []*box, x, y, w, h float64) {
		if w <= 0 || h <= 0 {
			return
		}
		var shown []*box
		for _, b := range children {
			if b.value > 0 {
				shown = append(shown, b)
			}
		}
		slices.SortStableFunc(shown, func(a, b *box) int {
			switch d := b.value - a.value; {
			case d < 0:
				return -1
			case d > 0:
				return 1
			}
			return 0
		})
		total := 0.0
		for _, b := range shown {
			total += b.value
		}
		if len(shown) == 0 || math.IsNaN(total) || math.IsInf(total, 0) {
			return
		}
		half := float64(gap / 2)
		squarify(shown, total, x-half, y-half, w+gap, h+gap, gap)
		for _, b := range shown {
			if b.w > 0 && b.h > 0 {
				placed = append(placed, b)
			}
		}
	}

	place(top, 0, 0, width, height)
	for i := 0; i < len(placed); i++ {
		b := placed[i]
		if len(b.children) > 0 {
			place(b.children, b.x+side, b.y+header, b.w-float64(2*side), b.h-header-side)
		}
	}
	return placed
}
