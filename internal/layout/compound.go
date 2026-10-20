package layout

import (
	"context"
	"math"
	"slices"
)

// Dir is the direction ranks run in.
type Dir string

// The directions. A CNode's Dir of "" is upstream's undefined: the group takes its parent's.
const (
	TB Dir = "TB"
	BT Dir = "BT"
	LR Dir = "LR"
	RL Dir = "RL"
)

// Direction is the direction a diagram names, or top to bottom for anything else.
func Direction(dir string) Dir {
	if dir == "BT" || dir == "LR" || dir == "RL" {
		return Dir(dir)
	}
	return TB
}

// CNode is a node of a compound graph. X and Y are outputs, and so are W and H for a group.
type CNode struct {
	W       float64
	H       float64
	Parent  int
	IsGroup bool
	Dir     Dir
	PadX    float64
	PadTop  float64
	// PadBottom is the room under a group's members.
	PadBottom float64
	MinW      float64
	// Declaration order; a group takes the smallest value among its members.
	Seq float64
	// How much of a side its edges may spread along, measured across the flow; 0 keeps them at its middle.
	Span float64
	X    float64
	Y    float64
}

// CEdge is an edge of a compound graph. Route, LabelX and LabelY are outputs.
type CEdge struct {
	Src    int
	Dst    int
	Minlen float64
	LabelW float64
	LabelH float64
	// x, y, axis triples; axis is 0 where the edge runs vertically into the point, 1 horizontally.
	Route  []float64
	LabelX float64
	LabelY float64
}

// CompoundResult is the size of a layout.
type CompoundResult struct {
	Width  float64
	Height float64
}

type side int

const (
	sideTop side = iota
	sideRight
	sideBottom
	sideLeft
)

type portEdge struct {
	e     int
	inner int
	out   bool
	side  side
	px    float64
	py    float64
	route []float64
}

type levelEdge struct {
	e int
	a int
	b int
}

type level struct {
	id     int
	dir    Dir
	items  []int
	edges  []levelEdge
	ports  []*portEdge
	portOf map[int]*portEdge
	routes map[int][]float64
	x      float64
	y      float64
}

const maxPorts = 40000

// ShiftLayout moves a laid-out graph by (ox, oy).
func ShiftLayout(nodes []CNode, edges []CEdge, ox, oy float64) {
	for i := range nodes {
		c := &nodes[i]
		c.X += ox
		c.Y += oy
	}
	for i := range edges {
		e := &edges[i]
		for k := 0; k < len(e.Route); k += 3 {
			e.Route[k] += ox
			e.Route[k+1] += oy
		}
		e.LabelX += ox
		e.LabelY += oy
	}
}

// NewCNode is upstream's cnode(); a parent of -1 is the root.
func NewCNode(w, h float64, parent int) CNode {
	return CNode{W: w, H: h, Parent: parent}
}

func startSide(dir Dir) side {
	switch dir {
	case TB:
		return sideTop
	case BT:
		return sideBottom
	case LR:
		return sideLeft
	}
	return sideRight
}

func opposite(s side) side {
	return (s + 2) & 3
}

// CompoundLayout lays out a graph whose nodes may be nested in groups. Each group is laid out on
// its own, in its own direction, and then placed in its parent as a single box. An edge that
// crosses a group's border passes through a port on the side facing its other end.
func CompoundLayout(ctx context.Context, nodes []CNode, edges []CEdge, rootDir Dir, opt LayeredOptions) CompoundResult {
	n := len(nodes)
	levelOf := map[int]*level{}
	order := []*level{}
	makeLevel := func(id int, dir Dir) *level {
		l := &level{id: id, dir: dir, portOf: map[int]*portEdge{}, routes: map[int][]float64{}}
		levelOf[id] = l
		return l
	}
	root := makeLevel(-1, rootDir)

	children := make([][]int, n)
	rootItems := []int{}
	for i := 0; i < n; i++ {
		p := nodes[i].Parent
		if p >= 0 && p < n && p != i && nodes[p].IsGroup {
			children[p] = append(children[p], i)
		} else {
			nodes[i].Parent = -1
			rootItems = append(rootItems, i)
		}
	}
	root.items = rootItems
	order = append(order, root)
	depth := make([]int, n)
	placed := make([]uint8, n)
	descend := func(from int) {
		for q := from; q < len(order); q++ {
			checkCancel(ctx)
			l := order[q]
			for _, item := range l.items {
				placed[item] = 1
				if l.id < 0 {
					depth[item] = 0
				} else {
					depth[item] = depth[l.id] + 1
				}
				if nodes[item].IsGroup {
					dir := nodes[item].Dir
					if dir == "" {
						dir = l.dir
					}
					child := makeLevel(item, dir)
					child.items = children[item]
					order = append(order, child)
				}
			}
		}
	}
	descend(0)
	// Groups that contain each other are unreachable from the root; lift them out.
	for i := 0; i < n; i++ {
		if placed[i] != 0 {
			continue
		}
		former := nodes[i].Parent
		kept := []int{}
		for _, c := range children[former] {
			if c != i {
				kept = append(kept, c)
			}
		}
		children[former] = kept
		nodes[i].Parent = -1
		root.items = append(root.items, i)
		placed[i] = 1
		depth[i] = 0
		if nodes[i].IsGroup {
			from := len(order)
			dir := nodes[i].Dir
			if dir == "" {
				dir = rootDir
			}
			child := makeLevel(i, dir)
			child.items = children[i]
			order = append(order, child)
			descend(from)
		}
	}

	for q := len(order) - 1; q > 0; q-- {
		l := order[q]
		seq := math.Inf(1)
		for _, item := range l.items {
			if nodes[item].Seq < seq {
				seq = nodes[item].Seq
			}
		}
		if !math.IsInf(seq, 1) {
			nodes[l.id].Seq = seq
		}
	}
	for _, l := range order {
		slices.SortStableFunc(l.items, func(a, b int) int {
			if c := sign(nodes[a].Seq - nodes[b].Seq); c != 0 {
				return c
			}
			return a - b
		})
	}

	// Every group border an edge crosses costs a port. Past the budget, an edge is drawn between
	// the groups that hold its ends, at the level they share.
	ports := 0
	for i := range edges {
		e := &edges[i]
		a := e.Src
		b := e.Dst
		crossed := 0
		for ; depth[a] > depth[b]; crossed++ {
			a = nodes[a].Parent
		}
		for ; depth[b] > depth[a]; crossed++ {
			b = nodes[b].Parent
		}
		for ; a != b && nodes[a].Parent != nodes[b].Parent; crossed += 2 {
			a = nodes[a].Parent
			b = nodes[b].Parent
		}
		if a == b {
			continue
		}
		if ports+crossed > maxPorts {
			e.Src = a
			e.Dst = b
		} else {
			ports += crossed
		}
	}

	loops := []int{}
	stray := []int{}
	for ei := range edges {
		checkCancel(ctx)
		e := &edges[ei]
		e.Route = []float64{}
		if e.Src == e.Dst {
			loops = append(loops, ei)
			continue
		}
		a := e.Src
		b := e.Dst
		aPath := []int{}
		bPath := []int{}
		for depth[a] > depth[b] {
			aPath = append(aPath, a)
			a = nodes[a].Parent
		}
		for depth[b] > depth[a] {
			bPath = append(bPath, b)
			b = nodes[b].Parent
		}
		if a == b {
			stray = append(stray, ei)
			continue
		}
		for nodes[a].Parent != nodes[b].Parent {
			aPath = append(aPath, a)
			bPath = append(bPath, b)
			a = nodes[a].Parent
			b = nodes[b].Parent
		}
		shared := levelOf[nodes[a].Parent]
		shared.edges = append(shared.edges, levelEdge{e: ei, a: a, b: b})
		for _, path := range [2]struct {
			items []int
			out   bool
		}{{aPath, true}, {bPath, false}} {
			for _, inner := range path.items {
				l := levelOf[nodes[inner].Parent]
				port := &portEdge{e: ei, inner: inner, out: path.out, side: sideTop, route: []float64{}}
				l.ports = append(l.ports, port)
				key := ei * 2
				if !path.out {
					key++
				}
				l.portOf[key] = port
			}
		}
	}

	local := make([]int, n)
	endpointBelow := func(item int, e *CEdge, out bool) bool {
		if out {
			return e.Src != item
		}
		return e.Dst != item
	}
	portKey := func(ei int, out bool) int {
		if out {
			return ei * 2
		}
		return ei*2 + 1
	}

	build := func(l *level, sized bool) ([]LNode, []LEdge, []int) {
		vertical := l.dir == TB || l.dir == BT
		start := startSide(l.dir)
		lnodes := make([]LNode, 0, len(l.items))
		ledges := make([]LEdge, 0, len(l.edges)+len(l.ports))
		for i, item := range l.items {
			local[item] = i
			node := &nodes[item]
			ln := NewLNode(0, 0, KindNode)
			if sized {
				if vertical {
					ln = NewLNode(node.W, node.H, KindNode)
				} else {
					ln = NewLNode(node.H, node.W, KindNode)
				}
			}
			if node.IsGroup {
				ln.Span = min(node.Span, ln.W-float64(2*node.PadX))
			} else {
				ln.Span = node.Span
			}
			lnodes = append(lnodes, ln)
		}
		held := func(item, ei int, out bool) bool {
			return nodes[item].IsGroup && endpointBelow(item, &edges[ei], out)
		}
		offset := func(item, ei int, out bool) float64 {
			if !sized || !nodes[item].IsGroup || !endpointBelow(item, &edges[ei], out) {
				return 0
			}
			port := levelOf[item].portOf[portKey(ei, out)]
			if vertical {
				return port.px - float64(nodes[item].W/2)
			}
			return port.py - float64(nodes[item].H/2)
		}
		for _, le := range l.edges {
			e := &edges[le.e]
			edge := NewLEdge(local[le.a], local[le.b], e.Minlen, 0, 0)
			if sized {
				if vertical {
					edge.LabelW, edge.LabelH = e.LabelW, e.LabelH
				} else {
					edge.LabelW, edge.LabelH = e.LabelH, e.LabelW
				}
				edge.TailDx = offset(le.a, le.e, true)
				edge.HeadDx = offset(le.b, le.e, false)
				edge.TailFixed = held(le.a, le.e, true)
				edge.HeadFixed = held(le.b, le.e, false)
			}
			ledges = append(ledges, edge)
		}
		dummies := fillInt(len(l.ports), -1)
		for i, port := range l.ports {
			if port.side == start || port.side == opposite(start) {
				d := len(lnodes)
				kind := KindEndPort
				if port.side == start {
					kind = KindStartPort
				}
				lnodes = append(lnodes, NewLNode(0, 0, kind))
				dummies[i] = d
				var edge LEdge
				if port.out {
					edge = NewLEdge(local[port.inner], d, 1, 0, 0)
					edge.TailDx = offset(port.inner, port.e, true)
					edge.TailFixed = held(port.inner, port.e, true)
				} else {
					edge = NewLEdge(d, local[port.inner], 1, 0, 0)
					edge.HeadDx = offset(port.inner, port.e, false)
					edge.HeadFixed = held(port.inner, port.e, false)
				}
				ledges = append(ledges, edge)
			} else if port.side == sideTop || port.side == sideLeft {
				lnodes[local[port.inner]].Pin = -1
			} else {
				lnodes[local[port.inner]].Pin = 1
			}
		}
		return lnodes, ledges, dummies
	}

	// Top-down: rank each level to learn which side of a group every crossing edge leaves from.
	for _, l := range order {
		checkCancel(ctx)
		for _, port := range l.ports {
			if nodes[port.inner].IsGroup && endpointBelow(port.inner, &edges[port.e], port.out) {
				levelOf[port.inner].portOf[portKey(port.e, port.out)].side = port.side
			}
		}
		if len(l.edges) == 0 {
			continue
		}
		needed := false
		for _, le := range l.edges {
			e := &edges[le.e]
			if (nodes[le.a].IsGroup && e.Src != le.a) || (nodes[le.b].IsGroup && e.Dst != le.b) {
				needed = true
			}
		}
		if !needed {
			continue
		}
		lnodes, ledges, _ := build(l, false)
		orient(ctx, lnodes, ledges)
		rank(ctx, lnodes, ledges, 1)
		start := startSide(l.dir)
		for i, le := range l.edges {
			e := &edges[le.e]
			aAbove := !ledges[i].Reversed
			if nodes[le.a].IsGroup && e.Src != le.a {
				s := start
				if aAbove {
					s = opposite(start)
				}
				levelOf[le.a].portOf[le.e*2].side = s
			}
			if nodes[le.b].IsGroup && e.Dst != le.b {
				s := opposite(start)
				if aAbove {
					s = start
				}
				levelOf[le.b].portOf[le.e*2+1].side = s
			}
		}
	}

	// Bottom-up: lay out each level now that the sizes of its groups are known.
	total := CompoundResult{}
	for q := len(order) - 1; q >= 0; q-- {
		l := order[q]
		var group *CNode
		if l.id >= 0 {
			group = &nodes[l.id]
		}
		vertical := l.dir == TB || l.dir == BT
		flip := l.dir == BT || l.dir == RL
		lnodes, ledges, dummies := build(l, true)
		size := Layered(ctx, lnodes, ledges, opt)
		contentW, contentH := size.Height, size.Width
		if vertical {
			contentW, contentH = size.Width, size.Height
		}
		var padX, padTop, padBottom, minW float64
		if group != nil {
			padX, padTop, padBottom, minW = group.PadX, group.PadTop, group.PadBottom, group.MinW
		}
		boxW := max(contentW+float64(2*padX), minW)
		boxH := contentH + padTop + padBottom
		offX := float64((boxW - contentW) / 2)
		offY := padTop
		tx := func(lx, ly float64) float64 {
			if vertical {
				return offX + lx
			}
			if flip {
				return offX + (size.Height - ly)
			}
			return offX + ly
		}
		ty := func(lx, ly float64) float64 {
			if !vertical {
				return offY + lx
			}
			if flip {
				return offY + (size.Height - ly)
			}
			return offY + ly
		}
		axis := 1.0
		if vertical {
			axis = 0
		}
		convert := func(points []float64) []float64 {
			out := make([]float64, 0, len(points)/2*3)
			for i := 0; i < len(points); i += 2 {
				out = append(out, tx(points[i], points[i+1]), ty(points[i], points[i+1]), axis)
			}
			return out
		}

		for i, item := range l.items {
			nodes[item].X = tx(lnodes[i].X, lnodes[i].Y)
			nodes[item].Y = ty(lnodes[i].X, lnodes[i].Y)
		}
		for i, le := range l.edges {
			edge := &ledges[i]
			l.routes[le.e] = convert(edge.Points)
			e := &edges[le.e]
			if e.LabelW > 0 {
				e.LabelX = tx(edge.LabelX, edge.LabelY)
				e.LabelY = ty(edge.LabelX, edge.LabelY)
			}
		}
		k := len(l.edges)
		for i, port := range l.ports {
			item := &nodes[port.inner]
			if dummies[i] >= 0 {
				route := convert(ledges[k].Points)
				k++
				at := 0
				if port.out {
					at = len(route) - 3
				}
				bx := route[at]
				by := route[at+1]
				switch port.side {
				case sideTop:
					by = 0
				case sideBottom:
					by = boxH
				case sideLeft:
					bx = 0
				default:
					bx = boxW
				}
				if port.out {
					route = append(route, bx, by, axis)
				} else {
					route = append([]float64{bx, by, axis}, route...)
				}
				port.px = bx
				port.py = by
				port.route = route
				continue
			}
			ix := item.X
			iy := item.Y
			if item.IsGroup && endpointBelow(port.inner, &edges[port.e], port.out) {
				inner := levelOf[port.inner].portOf[portKey(port.e, port.out)]
				ix += inner.px - float64(item.W/2)
				iy += inner.py - float64(item.H/2)
			} else {
				switch port.side {
				case sideTop:
					iy -= float64(item.H / 2)
				case sideBottom:
					iy += float64(item.H / 2)
				case sideLeft:
					ix -= float64(item.W / 2)
				default:
					ix += float64(item.W / 2)
				}
			}
			bx := ix
			by := iy
			switch port.side {
			case sideTop:
				by = 0
			case sideBottom:
				by = boxH
			case sideLeft:
				bx = 0
			default:
				bx = boxW
			}
			a := 1.0
			if port.side == sideTop || port.side == sideBottom {
				a = 0
			}
			port.px = bx
			port.py = by
			if port.out {
				port.route = []float64{ix, iy, a, bx, by, a}
			} else {
				port.route = []float64{bx, by, a, ix, iy, a}
			}
		}

		if group != nil {
			group.W = boxW
			group.H = boxH
		} else {
			total = CompoundResult{Width: boxW, Height: boxH}
		}
	}

	// Top-down again: turn positions inside each box into diagram coordinates.
	for _, l := range order {
		ox := l.x
		oy := l.y
		for _, item := range l.items {
			node := &nodes[item]
			node.X += ox
			node.Y += oy
			if node.IsGroup {
				child := levelOf[item]
				child.x = node.X - float64(node.W/2)
				child.y = node.Y - float64(node.H/2)
			}
		}
	}

	appendRoute := func(route, part []float64, ox, oy float64) []float64 {
		for i := 0; i < len(part); i += 3 {
			x := part[i] + ox
			y := part[i+1] + oy
			n := len(route)
			if n >= 3 && math.Abs(route[n-3]-x) < 0.01 && math.Abs(route[n-2]-y) < 0.01 {
				continue
			}
			route = append(route, x, y, part[i+2])
		}
		return route
	}

	for _, l := range order {
		checkCancel(ctx)
		for _, le := range l.edges {
			e := &edges[le.e]
			route := []float64{}
			outward := []*level{}
			for item := e.Src; nodes[item].Parent != l.id; item = nodes[item].Parent {
				outward = append(outward, levelOf[nodes[item].Parent])
			}
			for _, lv := range outward {
				route = appendRoute(route, lv.portOf[le.e*2].route, lv.x, lv.y)
			}
			route = appendRoute(route, l.routes[le.e], l.x, l.y)
			inward := []*level{}
			for item := e.Dst; nodes[item].Parent != l.id; item = nodes[item].Parent {
				inward = append(inward, levelOf[nodes[item].Parent])
			}
			for i := len(inward) - 1; i >= 0; i-- {
				route = appendRoute(route, inward[i].portOf[le.e*2+1].route, inward[i].x, inward[i].y)
			}
			e.Route = route
			if e.LabelW > 0 {
				e.LabelX += l.x
				e.LabelY += l.y
			}
		}
	}

	for _, ei := range stray {
		e := &edges[ei]
		a := &nodes[e.Src]
		b := &nodes[e.Dst]
		e.Route = []float64{a.X, a.Y, 0, b.X, b.Y, 0}
		e.LabelX = float64((a.X + b.X) / 2)
		e.LabelY = float64((a.Y + b.Y) / 2)
	}
	for _, ei := range loops {
		e := &edges[ei]
		node := &nodes[e.Src]
		e.LabelX = node.X + float64(node.W/2) + 28 + float64(e.LabelW/2)
		e.LabelY = node.Y
	}

	return total
}
