package block

import (
	"math"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

// cell is a box in the grid layout.
type cell struct {
	children []*cell
	// columns is the grid width of this container: a positive count, or -1 to put every child
	// on one row.
	columns float64
	span    float64
	space   bool
	// head is the height reserved above a container's grid for its title.
	head float64
	// nw and nh are the size the content needs. Set by the caller for leaves, computed here
	// for containers.
	nw, nh float64
	// Placement: grid position, then center and size in pixels.
	col, row   float64
	cols       float64
	heights    []float64
	x, y, w, h float64
}

const maxSpan = 1000

func newCell(nw, nh, span float64) *cell {
	return &cell{columns: -1, span: span, nw: nw, nh: nh}
}

func isInteger(v float64) bool { return !math.IsInf(v, 0) && v == math.Trunc(v) }

func calculateBlockPosition(columns, position float64) (px, py float64) {
	if columns == 0 || !isInteger(columns) {
		e := perr.New("Columns must be an integer !== 0.", perr.Semantic)
		e.Type = "block"
		perr.Throw(e)
	}
	if position < 0 || !isInteger(position) {
		e := perr.New("Position must be a non-negative integer."+jsnum.Format(position), perr.Semantic)
		e.Type = "block"
		perr.Throw(e)
	}
	if columns < 0 {
		return position, 0
	}
	return math.Mod(position, columns), math.Floor(position / columns)
}

// measure puts each child in its grid cell and sizes the container. Every column is as wide
// as the widest block needs. Every row that holds a plain block is as tall as the tallest
// block in the container, as in Mermaid; a row of nothing but containers is only as tall as
// they need.
func measure(box *cell, gap, inset float64) {
	columns := box.columns
	var heights []float64
	position := 0.0
	cols := 0.0
	cw := 0.0
	tallest := 0.0
	for _, child := range box.children {
		px, py := calculateBlockPosition(columns, position)
		// Mermaid lets a block wider than the rest of its row stick out of the grid. Here it stops at the last column.
		span := 1.0
		if child.span >= 1 {
			span = min(child.span, maxSpan)
		}
		if columns > 0 && span > columns-px {
			span = columns - px
		}
		child.span = span
		child.col = px
		child.row = py
		position += span
		if px+span > cols {
			cols = px + span
		}
		row := int(py)
		if row == len(heights) {
			heights = append(heights, 0)
		}
		if len(child.children) == 0 {
			heights[row] = -1
		} else if heights[row] >= 0 && child.nh > heights[row] {
			heights[row] = child.nh
		}
		if child.space {
			continue
		}
		if width := (child.nw - float64(gap*(span-1))) / span; width > cw {
			cw = width
		}
		if child.nh > tallest {
			tallest = child.nh
		}
	}
	height := float64(float64(len(heights)-1) * gap)
	for row := range heights {
		if heights[row] < 0 {
			heights[row] = tallest
		}
		height += heights[row]
	}
	box.cols = cols
	box.heights = heights
	box.nw = max(box.nw, float64(cols*cw)+float64((cols-1)*gap)+float64(2*inset))
	box.nh = height + float64(2*inset) + box.head
}

// arrange gives each child its cell, sharing out any room the container has beyond what it
// needs.
func arrange(box *cell, gap, inset float64) {
	cols, heights := box.cols, box.heights
	rows := float64(len(heights))
	cw := (box.w - float64(2*inset) - float64((cols-1)*gap)) / cols
	room := box.h - float64(2*inset) - box.head - float64((rows-1)*gap)
	needed := 0.0
	for _, height := range heights {
		needed += height
	}
	scale, share := 0.0, 0.0
	if needed > 0 {
		scale = room / needed
	} else {
		share = room / rows
	}
	left := box.x - float64(box.w/2) + inset
	top := box.y - float64(box.h/2) + inset + box.head
	row := 0
	for _, child := range box.children {
		for ; float64(row) < child.row; row++ {
			top += float64(heights[row]*scale) + share + gap
		}
		child.w = float64(cw*child.span) + float64(gap*(child.span-1))
		child.h = float64(heights[row]*scale) + share
		child.x = left + float64(child.col*(cw+gap)) + float64(child.w/2)
		child.y = top + float64(child.h/2)
	}
}

// layout lays out a tree of cells in place. The root has no border, so its grid starts at the
// origin.
func layout(root *cell, gap float64) (width, height float64) {
	order := []*cell{root}
	for i := 0; i < len(order); i++ {
		order = append(order, order[i].children...)
	}
	insetOf := func(i int) float64 {
		if i == 0 {
			return 0
		}
		return gap
	}
	for i := len(order) - 1; i >= 0; i-- {
		if box := order[i]; len(box.children) > 0 {
			measure(box, gap, insetOf(i))
		}
	}
	root.w = root.nw
	root.h = root.nh
	root.x = float64(root.w / 2)
	root.y = float64(root.h / 2)
	for i, box := range order {
		if len(box.children) > 0 {
			arrange(box, gap, insetOf(i))
		}
	}
	return root.w, root.h
}
