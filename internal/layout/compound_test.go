package layout_test

import (
	"context"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/layout"
)

// Ported from upstream's tests/layout/compound.test.ts.

var testOptions = layout.LayeredOptions{NodeSep: 40, EdgeSep: 16, RankSep: 48, PortSep: 20}

var dirs = []layout.Dir{layout.TB, layout.BT, layout.LR, layout.RL}

// random is upstream's tests/support/corpus.ts random(): a seeded linear congruential generator.
func random(seed uint32) func() float64 {
	state := seed
	return func() float64 {
		state = state*1664525 + 1013904223
		return float64(state) / 4294967296
	}
}

func edge(src, dst int, label ...float64) layout.CEdge {
	e := layout.CEdge{Src: src, Dst: dst, Minlen: 1, Route: []float64{}}
	if len(label) == 2 {
		e.LabelW, e.LabelH = label[0], label[1]
	}
	return e
}

// closeTo is vitest's toBeCloseTo(want, digits).
func closeTo(got, want float64, digits int) bool {
	return math.Abs(got-want) < math.Pow(10, float64(-digits))/2
}

func requireClose(t *testing.T, got, want float64, what string) {
	t.Helper()
	if !closeTo(got, want, 5) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// Every other graph lets edges spread along the sides of its nodes.
func randomCompound(seed uint32) ([]layout.CNode, []layout.CEdge, layout.Dir) {
	rnd := random(seed)
	intn := func(n int) int { return int(math.Floor(rnd() * float64(n))) }
	nodes := []layout.CNode{}
	groups := []int{}
	groupCount := intn(5)
	for g := 0; g < groupCount; g++ {
		parent := -1
		if len(groups) > 0 && rnd() < 0.4 {
			parent = groups[intn(len(groups))]
		}
		node := layout.NewCNode(0, 0, parent)
		node.IsGroup = true
		node.PadX = 20
		node.PadTop = 40
		node.PadBottom = 20
		node.MinW = 60
		if rnd() < 0.3 {
			node.Dir = dirs[intn(4)]
		}
		node.Seq = float64(1000 + g)
		if seed%2 == 0 {
			node.Span = math.Inf(1)
		}
		groups = append(groups, len(nodes))
		nodes = append(nodes, node)
	}
	leaves := 3 + intn(30)
	for i := 0; i < leaves; i++ {
		parent := -1
		if len(groups) > 0 && rnd() < 0.5 {
			parent = groups[intn(len(groups))]
		}
		node := layout.NewCNode(float64(30+intn(120)), float64(30+intn(50)), parent)
		node.Seq = float64(i)
		if seed%2 == 0 {
			node.Span = min(node.W, node.H) - 24
		}
		nodes = append(nodes, node)
	}
	edges := []layout.CEdge{}
	count := intn(leaves * 2)
	for i := 0; i < count; i++ {
		labelled := rnd() < 0.3
		src, dst := intn(len(nodes)), intn(len(nodes))
		if labelled {
			edges = append(edges, edge(src, dst, float64(20+intn(60)), 24))
		} else {
			edges = append(edges, edge(src, dst))
		}
	}
	return nodes, edges, dirs[intn(4)]
}

func isAncestor(nodes []layout.CNode, a, b int) bool {
	for p := nodes[b].Parent; p >= 0; p = nodes[p].Parent {
		if p == a {
			return true
		}
	}
	return false
}

func onBorder(node layout.CNode, x, y float64) bool {
	dx := math.Abs(x - node.X)
	dy := math.Abs(y - node.Y)
	inside := dx <= node.W/2+0.5 && dy <= node.H/2+0.5
	edgeX := math.Abs(dx-node.W/2) <= 0.5
	edgeY := math.Abs(dy-node.H/2) <= 0.5
	return inside && (edgeX || edgeY)
}

func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func seqNodes(nodes []layout.CNode) {
	for i := range nodes {
		nodes[i].Seq = float64(i)
	}
}

func cnodes(n int, w, h float64) []layout.CNode {
	nodes := make([]layout.CNode, n)
	for i := range nodes {
		nodes[i] = layout.NewCNode(w, h, -1)
	}
	return nodes
}

func TestCompoundKeepsRandomGraphsWellFormed(t *testing.T) {
	for seed := uint32(1); seed <= 300; seed++ {
		nodes, edges, dir := randomCompound(seed)
		size := layout.CompoundLayout(context.Background(), nodes, edges, dir, testOptions)
		where := fmt.Sprintf("seed %d", seed)
		if !finite(size.Width, size.Height) {
			t.Fatalf("%s: size not finite", where)
		}
		for i, node := range nodes {
			if !finite(node.X, node.Y) {
				t.Errorf("%s node %d: not finite", where, i)
			}
			if node.X-node.W/2 < -0.5 || node.Y-node.H/2 < -0.5 || node.X+node.W/2 > size.Width+0.5 || node.Y+node.H/2 > size.Height+0.5 {
				t.Errorf("%s node %d: outside the drawing", where, i)
			}
			if node.Parent >= 0 {
				parent := nodes[node.Parent]
				if math.Abs(node.X-parent.X)+node.W/2 > parent.W/2+0.5 || math.Abs(node.Y-parent.Y)+node.H/2 > parent.H/2+0.5 {
					t.Errorf("%s node %d: outside its parent", where, i)
				}
			}
		}
		for a := range nodes {
			for b := a + 1; b < len(nodes); b++ {
				if nodes[a].Parent != nodes[b].Parent {
					continue
				}
				overlapX := (nodes[a].W+nodes[b].W)/2 - math.Abs(nodes[a].X-nodes[b].X)
				overlapY := (nodes[a].H+nodes[b].H)/2 - math.Abs(nodes[a].Y-nodes[b].Y)
				if overlapX > 0.5 && overlapY > 0.5 {
					t.Errorf("%s nodes %d and %d overlap", where, a, b)
				}
			}
		}
		for i, e := range edges {
			if e.Src == e.Dst {
				continue
			}
			if len(e.Route)%3 != 0 || len(e.Route) < 6 || !finite(e.Route...) {
				t.Errorf("%s edge %d: bad route %v", where, i, e.Route)
				continue
			}
			if isAncestor(nodes, e.Src, e.Dst) || isAncestor(nodes, e.Dst, e.Src) {
				continue
			}
			last := len(e.Route) - 3
			if !onBorder(nodes[e.Src], e.Route[0], e.Route[1]) {
				t.Errorf("%s edge %d: does not start on its source", where, i)
			}
			if !onBorder(nodes[e.Dst], e.Route[last], e.Route[last+1]) {
				t.Errorf("%s edge %d: does not end on its target", where, i)
			}
		}
	}
}

func TestCompoundIsDeterministic(t *testing.T) {
	for seed := uint32(1); seed <= 40; seed++ {
		n1, e1, d1 := randomCompound(seed)
		n2, e2, d2 := randomCompound(seed)
		layout.CompoundLayout(context.Background(), n1, e1, d1, testOptions)
		layout.CompoundLayout(context.Background(), n2, e2, d2, testOptions)
		if !slices.Equal(n1, n2) {
			t.Errorf("seed %d: nodes differ", seed)
		}
		for i := range e1 {
			if !slices.Equal(e1[i].Route, e2[i].Route) {
				t.Errorf("seed %d edge %d: routes differ", seed, i)
			}
		}
	}
}

func TestCompoundRunsRanksAlongTheRequestedDirection(t *testing.T) {
	for _, dir := range dirs {
		nodes := cnodes(3, 60, 40)
		seqNodes(nodes)
		layout.CompoundLayout(context.Background(), nodes, []layout.CEdge{edge(0, 1), edge(1, 2)}, dir, testOptions)
		axis := func(c layout.CNode) float64 {
			if dir == layout.TB || dir == layout.BT {
				return c.Y
			}
			return c.X
		}
		sign := 1.0
		if dir == layout.BT || dir == layout.RL {
			sign = -1
		}
		if sign*(axis(nodes[1])-axis(nodes[0])) <= 40 || sign*(axis(nodes[2])-axis(nodes[1])) <= 40 {
			t.Errorf("%s: ranks do not run along the direction", dir)
		}
	}
}

func TestCompoundKeepsALongEdgeStraightThroughTheRanksItSkips(t *testing.T) {
	nodes := cnodes(3, 60, 40)
	seqNodes(nodes)
	edges := []layout.CEdge{edge(0, 1), edge(1, 2), edge(0, 2)}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	route := edges[2].Route
	xs := map[float64]bool{}
	for i := 3; i < len(route)-3; i += 3 {
		xs[jsmath.Round(route[i])] = true
	}
	if len(xs) != 1 {
		t.Errorf("long edge bends: %v", route)
	}
}

// turns counts how many times a route changes its place across the flow.
func turns(route []float64) int {
	count := 0
	for i := 3; i < len(route); i += 3 {
		if math.Abs(route[i]-route[i-3]) > 0.01 {
			count++
		}
	}
	return count
}

func TestCompoundRunsAnEdgeThatSkipsRanksInLineWithOneOfItsEnds(t *testing.T) {
	// A chain of eight, with edges that skip ranks down it and back up.
	nodes := cnodes(8, 80, 40)
	for i := range nodes {
		nodes[i].Seq = float64(i)
		nodes[i].Span = 56
	}
	edges := []layout.CEdge{edge(0, 1), edge(1, 2), edge(2, 3), edge(3, 4), edge(4, 5), edge(5, 6), edge(6, 7), edge(0, 3), edge(0, 7), edge(1, 5), edge(2, 6), edge(6, 1), edge(7, 0)}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	for _, e := range edges[7:] {
		if got := turns(e.Route); got != 1 {
			t.Errorf("%d to %d turns %d times, want 1", e.Src, e.Dst, got)
		}
	}
	// The edge from the second node down and the one back up to it both meet it in line.
	down := edges[9].Route
	up := edges[11].Route
	requireClose(t, down[3], down[0], "down edge's first bend")
	requireClose(t, up[len(up)-6], up[len(up)-3], "up edge's last bend")
}

func TestCompoundKeepsTheEdgesOnOneSideAtItsMiddleUnlessTheNodeHasASpan(t *testing.T) {
	nodes := []layout.CNode{layout.NewCNode(120, 40, -1), layout.NewCNode(60, 40, -1), layout.NewCNode(60, 40, -1), layout.NewCNode(60, 40, -1)}
	seqNodes(nodes)
	edges := []layout.CEdge{edge(0, 1), edge(0, 2), edge(0, 3)}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	for _, e := range edges {
		requireClose(t, e.Route[0], nodes[0].X, "start")
	}
}

func TestCompoundSpreadsTheEdgesOnOneSideInTheOrderOfTheNodesTheyLeadTo(t *testing.T) {
	for _, dir := range dirs {
		nodes := []layout.CNode{layout.NewCNode(120, 120, -1), layout.NewCNode(60, 60, -1), layout.NewCNode(60, 60, -1), layout.NewCNode(60, 60, -1)}
		seqNodes(nodes)
		nodes[0].Span = 96
		edges := []layout.CEdge{edge(0, 1), edge(0, 2), edge(0, 3)}
		layout.CompoundLayout(context.Background(), nodes, edges, dir, testOptions)
		vertical := dir == layout.TB || dir == layout.BT
		starts := make([]float64, 3)
		targets := make([]float64, 3)
		for i, e := range edges {
			if vertical {
				starts[i] = e.Route[0] - nodes[0].X
				targets[i] = nodes[e.Dst].X
			} else {
				starts[i] = e.Route[1] - nodes[0].Y
				targets[i] = nodes[e.Dst].Y
			}
		}
		byStart := []int{0, 1, 2}
		slices.SortStableFunc(byStart, func(a, b int) int { return int(math.Copysign(1, starts[a]-starts[b])) })
		byTarget := []int{0, 1, 2}
		slices.SortStableFunc(byTarget, func(a, b int) int { return int(math.Copysign(1, targets[a]-targets[b])) })
		if !slices.Equal(byStart, byTarget) {
			t.Errorf("%s: ends %v not in the order of their targets %v", dir, byStart, byTarget)
		}
		// None of the three can run straight, so they stay around the middle, a pitch apart.
		requireClose(t, starts[byStart[0]], -16, string(dir)+" first end")
		requireClose(t, starts[byStart[1]], 0, string(dir)+" second end")
		requireClose(t, starts[byStart[2]], 16, string(dir)+" third end")
		for _, e := range edges {
			if !onBorder(nodes[0], e.Route[0], e.Route[1]) {
				t.Errorf("%s: an end is off the node's border", dir)
			}
		}
	}
}

func TestCompoundKeepsAnEdgeThatArrivesApartFromOneThatLeavesOnTheSameSide(t *testing.T) {
	nodes := cnodes(2, 60, 40)
	for i := range nodes {
		nodes[i].Seq = float64(i)
		nodes[i].Span = 36
	}
	edges := []layout.CEdge{edge(0, 1), edge(1, 0)}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	down, up := edges[0].Route, edges[1].Route
	requireClose(t, math.Abs(down[len(down)-3]-up[0]), 16, "gap at the lower node")
	requireClose(t, math.Abs(down[0]-up[len(up)-3]), 16, "gap at the upper node")
}

func TestCompoundSharesAPlaceAmongEdgesThatRunTheSameWayWhenThereIsNoRoom(t *testing.T) {
	nodes := cnodes(13, 60, 40)
	for i := range nodes {
		nodes[i].Seq = float64(i)
		nodes[i].Span = 36
	}
	edges := []layout.CEdge{}
	for i := 0; i < 12; i++ {
		edges = append(edges, edge(i, 12))
	}
	// One edge the other way on the same side, which keeps a place of its own.
	edges = append(edges, edge(12, 0))
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	places := map[float64]bool{}
	back := edges[12].Route
	for _, e := range edges[:12] {
		x := e.Route[len(e.Route)-3]
		places[jsmath.Round(x*100)] = true
		if math.Abs(x-back[0]) < 8 {
			t.Errorf("an arriving end is %v from the leaving one", math.Abs(x-back[0]))
		}
	}
	if len(places) > 2 {
		t.Errorf("arriving ends take %d places, want at most 2", len(places))
	}
	requireClose(t, back[1], nodes[12].Y-20, "leaving end")
}

func TestCompoundStandsARankThatOnlySendsOnOneLine(t *testing.T) {
	lay := func(span float64) []layout.CNode {
		nodes := []layout.CNode{layout.NewCNode(60, 40, -1), layout.NewCNode(60, 120, -1), layout.NewCNode(60, 30, -1), layout.NewCNode(60, 90, -1)}
		for i := range nodes {
			nodes[i].Seq = float64(i)
			nodes[i].Span = span
		}
		layout.CompoundLayout(context.Background(), nodes, []layout.CEdge{edge(0, 2), edge(1, 2), edge(0, 3), edge(1, 3)}, layout.TB, testOptions)
		return nodes
	}
	spread := lay(36)
	requireClose(t, spread[0].Y+20, spread[1].Y+60, "bottoms of the sending rank")
	requireClose(t, spread[2].Y-15, spread[3].Y-45, "tops of the receiving rank")
	// Without spreading, as in the diagram types that do not use it yet, each rank is centered.
	plain := lay(0)
	requireClose(t, plain[0].Y, plain[1].Y, "centers of the first rank")
	requireClose(t, plain[2].Y, plain[3].Y, "centers of the second rank")
}

// fan is a node above a row of `children`, with an edge to each. With a span, the edges
// spread along the nodes' sides.
func fan(children int, span float64) ([]layout.CNode, []layout.CEdge, float64) {
	nodes := []layout.CNode{layout.NewCNode(100, 40, -1)}
	edges := []layout.CEdge{}
	for i := 1; i <= children; i++ {
		nodes = append(nodes, layout.NewCNode(100, 40, -1))
		edges = append(edges, edge(0, i))
	}
	for i := range nodes {
		nodes[i].Seq = float64(i)
		nodes[i].Span = span
	}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	return nodes, edges, nodes[1].Y - nodes[0].Y - 40
}

func TestCompoundGivesTheGapMoreRoomWhenEdgesRunFarAcross(t *testing.T) {
	_, _, gap := fan(3, 0)
	requireClose(t, gap, testOptions.RankSep, "gap with 3")
	// Seven children reach 420 across from the middle, and the gap is an eighth of that.
	_, _, gap = fan(7, 0)
	requireClose(t, gap, 420.0/8, "gap with 7")
	_, _, gap = fan(40, 0)
	requireClose(t, gap, 3*testOptions.RankSep, "gap with 40")
}

func TestCompoundTurnsEdgesThatRunFarAcrossAtLevelsOfTheirOwn(t *testing.T) {
	// With three children nothing runs far, and each edge is one curve across the usual gap.
	_, few, gap := fan(3, 76)
	requireClose(t, gap, testOptions.RankSep, "gap with 3")
	for _, e := range few {
		if len(e.Route) != 6 {
			t.Errorf("route %v, want one curve", e.Route)
		}
	}
	// With seven, the three on each side run down, across and down again.
	nodes, edges, gap := fan(7, 76)
	level := func(e layout.CEdge) float64 {
		t.Helper()
		if len(e.Route) != 12 {
			t.Fatalf("route %v, want four points", e.Route)
		}
		requireClose(t, e.Route[3], e.Route[0], "down")
		requireClose(t, e.Route[7], e.Route[4], "across")
		requireClose(t, e.Route[9], e.Route[6], "down again")
		return e.Route[4]
	}
	// The edge that runs furthest turns first, and each of the others 12 further down.
	requireClose(t, level(edges[1])-level(edges[0]), 12, "second level")
	requireClose(t, level(edges[2])-level(edges[1]), 12, "third level")
	for i := 0; i < 3; i++ {
		requireClose(t, level(edges[6-i]), level(edges[i]), "mirrored level")
	}
	// The one in the middle runs straight down.
	if len(edges[3].Route) != 6 {
		t.Errorf("middle route %v, want one line", edges[3].Route)
	}
	requireClose(t, edges[3].Route[3], edges[3].Route[0], "middle edge")
	// The levels keep 28 from the ranks above and below.
	requireClose(t, level(edges[0])-nodes[0].Y-20, 28, "first level from the rank")
	requireClose(t, gap, 2*28+2*12, "gap")
}

func TestCompoundDrawsCrossingEdgesAsPlainCurvesInATallerGap(t *testing.T) {
	// Four nodes above four, each with an edge to every one below.
	nodes := cnodes(8, 80, 40)
	for i := range nodes {
		nodes[i].Seq = float64(i)
		nodes[i].Span = 56
	}
	edges := []layout.CEdge{}
	for i := 0; i < 4; i++ {
		for j := 4; j < 8; j++ {
			edges = append(edges, edge(i, j))
		}
	}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	widest := 0.0
	for _, e := range edges {
		// No edge turns onto a level: each runs from one point to the other.
		if len(e.Route) != 6 {
			t.Errorf("route %v, want one curve", e.Route)
			continue
		}
		widest = max(widest, math.Abs(e.Route[0]-e.Route[3]))
	}
	// The gap is half as tall as the widest edge runs across, and no more than three times the usual.
	if widest <= 6*testOptions.RankSep {
		t.Errorf("widest edge runs %v across", widest)
	}
	requireClose(t, nodes[4].Y-nodes[0].Y-40, 3*testOptions.RankSep, "gap")
}

func TestCompoundSetsARankOfLabelsHalfAnArrowheadNearerTheTails(t *testing.T) {
	lay := func(headRoom float64) []layout.CEdge {
		nodes := cnodes(2, 60, 40)
		for i := range nodes {
			nodes[i].Seq = float64(i)
			nodes[i].Span = 36
		}
		edges := []layout.CEdge{edge(0, 1, 40, 20), edge(0, 1, 50, 20), edge(0, 1)}
		opt := testOptions
		opt.HeadRoom = headRoom
		layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, opt)
		return edges
	}
	// Without arrowheads the labels are midway between the two nodes.
	plain := lay(0)
	requireClose(t, plain[0].LabelY, (40+108)/2.0, "label without head room")
	// With them, the labels are midway between the node above and the backs of the arrowheads.
	edges := lay(8)
	first, second, bare := edges[0], edges[1], edges[2]
	requireClose(t, first.LabelY, (40+108-8)/2.0, "first label")
	requireClose(t, second.LabelY, first.LabelY, "second label")
	// A labelled edge runs straight for the height of its label, and the bare one for half of that.
	requireClose(t, first.Route[7]-first.Route[4], 20, "labelled run")
	requireClose(t, bare.Route[7]-bare.Route[4], 10, "bare run")
	requireClose(t, (bare.Route[4]+bare.Route[7])/2, first.LabelY, "bare run's middle")
}

func TestCompoundTurnsAFarEdgeAtALevelWhereAShorterOneWouldRunBesideIt(t *testing.T) {
	// A node above a row of three, and an edge back up to it from below the row.
	nodes := cnodes(5, 80, 40)
	for i := range nodes {
		nodes[i].Seq = float64(i)
		nodes[i].Span = 56
	}
	edges := []layout.CEdge{edge(0, 1), edge(0, 2), edge(0, 3), edge(1, 4), edge(2, 4), edge(3, 4), edge(4, 0)}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	// The edge back up passes the row on one side, then runs across to the top node and up into it.
	back := edges[6].Route
	n := len(back)
	level := back[n-5]
	requireClose(t, back[n-8], level, "run across")
	requireClose(t, back[n-6], back[n-3], "run up")
	if math.Abs(back[n-9]-back[n-6]) <= 2*testOptions.RankSep {
		t.Errorf("back edge runs only %v across", math.Abs(back[n-9]-back[n-6]))
	}
	// The edge down to the node on that side leaves next to where it arrives. It runs straight
	// down as far as that level and curves below it.
	down := edges[2].Route
	if math.Abs(down[0]-back[n-3]) >= 20 {
		t.Errorf("down edge leaves %v from the back edge", math.Abs(down[0]-back[n-3]))
	}
	if len(down) != 9 {
		t.Fatalf("down route %v, want three points", down)
	}
	requireClose(t, down[3], down[0], "down edge")
	if down[4] <= level-0.01 {
		t.Errorf("down edge curves above the level")
	}
}

func TestCompoundMovesAnEndAlongItsSideToLetItsEdgeRunStraight(t *testing.T) {
	// A wide node above four narrow ones.
	nodes := []layout.CNode{layout.NewCNode(400, 40, -1), layout.NewCNode(40, 40, -1), layout.NewCNode(40, 40, -1), layout.NewCNode(40, 40, -1), layout.NewCNode(40, 40, -1)}
	for i := range nodes {
		nodes[i].Seq = float64(i)
		nodes[i].Span = nodes[i].W - 24
	}
	edges := []layout.CEdge{edge(0, 1), edge(0, 2), edge(0, 3), edge(0, 4)}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	// Each child is under the wide node, so each edge runs straight down to it.
	for _, e := range edges {
		requireClose(t, e.Route[0], e.Route[len(e.Route)-3], "straight edge")
	}
	twoWay := []layout.CNode{layout.NewCNode(200, 40, -1), layout.NewCNode(40, 40, -1)}
	for i := range twoWay {
		twoWay[i].Seq = float64(i)
		twoWay[i].Span = twoWay[i].W - 24
	}
	pair := []layout.CEdge{edge(0, 1), edge(1, 0)}
	layout.CompoundLayout(context.Background(), twoWay, pair, layout.TB, testOptions)
	requireClose(t, math.Abs(pair[0].Route[0]-pair[1].Route[len(pair[1].Route)-3]), 16, "pair gap")
}

func TestCompoundMovesEdgesThatLeaveSideBySideCloserBeforeTheyShareAPlace(t *testing.T) {
	nodes := []layout.CNode{layout.NewCNode(60, 40, -1)}
	edges := []layout.CEdge{}
	for i := 1; i <= 7; i++ {
		nodes = append(nodes, layout.NewCNode(60, 40, -1))
		edges = append(edges, edge(0, i))
	}
	seqNodes(nodes)
	nodes[0].Span = 36
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	starts := []float64{}
	for _, e := range edges {
		starts = append(starts, e.Route[0])
	}
	slices.Sort(starts)
	// Six gaps of half a pitch each across the 36 there is.
	for i := 1; i < len(starts); i++ {
		requireClose(t, starts[i]-starts[i-1], 6, "gap between starts")
	}
}

func TestCompoundDrawsAnEdgeAsOneCurveAcrossARankThatOnlyATwinPutThere(t *testing.T) {
	lay := func(twin bool) []layout.CEdge {
		nodes := cnodes(4, 60, 40)
		for i := range nodes {
			nodes[i].Seq = float64(i)
			nodes[i].Span = 36
		}
		edges := []layout.CEdge{edge(0, 1), edge(0, 2), edge(0, 3)}
		if twin {
			edges = append(edges, edge(0, 3))
		}
		layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
		return edges
	}
	plain := lay(false)
	twinned := lay(true)
	// The twins make every edge take a rank in the middle, but none of them needs a bend there.
	for _, e := range twinned[:2] {
		if len(e.Route) != 6 {
			t.Errorf("route %v, want one curve", e.Route)
		}
	}
	requireClose(t, twinned[0].Route[1], plain[0].Route[1], "start")
	// The gap is tall enough for the twins to turn at two levels, 12 apart and 28 from the ranks.
	requireClose(t, twinned[0].Route[4], plain[0].Route[4]+2*28+12-testOptions.RankSep, "end")
	// The twins leave and arrive at places of their own.
	one, other := twinned[2].Route, twinned[3].Route
	if math.Abs(one[0]-other[0]) < 8 || math.Abs(one[len(one)-3]-other[len(other)-3]) < 8 {
		t.Errorf("twins share a place")
	}
	// They would run close beside each other, so one turns straight away and the other further down.
	turnsAt := func(route []float64) float64 {
		if math.Abs(route[3]-route[0]) < 0.01 {
			return route[4]
		}
		return route[1]
	}
	if math.Abs(turnsAt(one)-turnsAt(other)) < 10 {
		t.Errorf("twins turn %v apart", math.Abs(turnsAt(one)-turnsAt(other)))
	}
}

func TestCompoundKeepsTwinEdgesApartWhereTheirEndsCannotBe(t *testing.T) {
	nodes := cnodes(2, 60, 40)
	seqNodes(nodes)
	edges := []layout.CEdge{edge(0, 1), edge(0, 1)}
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	a, b := edges[0].Route, edges[1].Route
	if len(a) != 9 || len(b) != 9 {
		t.Fatalf("routes %v and %v, want three points each", a, b)
	}
	requireClose(t, a[0], b[0], "starts")
	requireClose(t, math.Abs(a[3]-b[3]), testOptions.EdgeSep, "bends apart")
}

func TestCompoundRoutesEdgesAtRightAnglesOnTracks(t *testing.T) {
	type run struct{ y, lo, hi, from, to float64 }
	for seed := uint32(1); seed <= 120; seed++ {
		rnd := random(seed * 7919)
		intn := func(n float64) int { return int(math.Floor(rnd() * n)) }
		nodes := []layout.CNode{}
		count := 3 + intn(14)
		for i := 0; i < count; i++ {
			node := layout.NewCNode(float64(40+intn(120)), float64(30+intn(60)), -1)
			node.Seq = float64(i)
			node.Span = node.W - 24
			nodes = append(nodes, node)
		}
		edges := []layout.CEdge{}
		for i, wanted := 0, intn(float64(count)*2.5); i < wanted; i++ {
			src := intn(float64(count))
			dst := intn(float64(count))
			if src != dst {
				labelW := 0.0
				if rnd() < 0.2 {
					labelW = float64(30 + intn(40))
				}
				edges = append(edges, edge(src, dst, labelW, 20))
			}
		}
		opt := testOptions
		opt.Tracks = true
		layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, opt)
		where := fmt.Sprintf("seed %d", seed)
		// Every run across, with the x where the edge comes down to it and where it goes down from it.
		runs := []run{}
		for ei, e := range edges {
			r := e.Route
			if !onBorder(nodes[e.Src], r[0], r[1]) {
				t.Errorf("%s edge %d: does not start on its source", where, ei)
			}
			if !onBorder(nodes[e.Dst], r[len(r)-3], r[len(r)-2]) {
				t.Errorf("%s edge %d: does not end on its target", where, ei)
			}
			for k := 3; k < len(r); k += 3 {
				// Two ends less than half a unit out of line are joined directly.
				across := math.Abs(r[k+1]-r[k-2]) < 0.01
				down := math.Abs(r[k]-r[k-3]) < 0.5
				if !across && !down {
					t.Errorf("%s edge %d: turns at other than a right angle", where, ei)
				}
				if !across || down {
					continue
				}
				// The route runs upwards when the edge does, so the higher end is the one it comes down to.
				before := r[k-2]
				if k >= 6 {
					before = r[k-5]
				}
				fromFirst := before < r[k-2]-0.01 || (k+4 < len(r) && r[k+4] > r[k+1]+0.01)
				from, to := r[k], r[k-3]
				if fromFirst {
					from, to = r[k-3], r[k]
				}
				runs = append(runs, run{y: r[k+1], lo: min(r[k], r[k-3]), hi: max(r[k], r[k-3]), from: from, to: to})
			}
		}
		for a := range runs {
			for b := a + 1; b < len(runs); b++ {
				if math.Abs(runs[a].y-runs[b].y) > 0.01 {
					continue
				}
				overlap := runs[a].lo < runs[b].hi-0.5 && runs[b].lo < runs[a].hi-0.5
				mates := math.Abs(runs[a].from-runs[b].from) < 0.5 || math.Abs(runs[a].to-runs[b].to) < 0.5
				if overlap && !mates {
					t.Errorf("%s runs %d and %d lie on one track", where, a, b)
				}
			}
		}
		// No run across passes through a node.
		for _, r := range runs {
			for _, node := range nodes {
				if math.Abs(r.y-node.Y) < node.H/2-0.5 && r.lo < node.X+node.W/2-0.5 && r.hi > node.X-node.W/2+0.5 {
					t.Errorf("%s a run crosses a node", where)
				}
			}
		}
	}
}

func TestCompoundNestsTheTracksOfAFan(t *testing.T) {
	nodes := []layout.CNode{layout.NewCNode(120, 40, -1), layout.NewCNode(60, 40, -1), layout.NewCNode(60, 40, -1), layout.NewCNode(60, 40, -1), layout.NewCNode(60, 40, -1)}
	seqNodes(nodes)
	nodes[0].Span = 96
	edges := []layout.CEdge{edge(0, 1), edge(0, 2), edge(0, 3), edge(0, 4)}
	opt := testOptions
	opt.Tracks = true
	layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, opt)
	var left, right []layout.CEdge
	for _, e := range edges {
		if nodes[e.Dst].X < nodes[0].X {
			left = append(left, e)
		} else if nodes[e.Dst].X > nodes[0].X {
			right = append(right, e)
		}
	}
	slices.SortStableFunc(left, func(a, b layout.CEdge) int { return int(math.Copysign(1, nodes[a.Dst].X-nodes[b.Dst].X)) })
	slices.SortStableFunc(right, func(a, b layout.CEdge) int { return int(math.Copysign(1, nodes[b.Dst].X-nodes[a.Dst].X)) })
	// The edge that reaches furthest turns first, so the nearer ones pass under it.
	for _, side := range [][]layout.CEdge{left, right} {
		for i := 1; i < len(side); i++ {
			if side[i-1].Route[4] >= side[i].Route[4] {
				t.Errorf("tracks of the fan do not nest")
			}
		}
	}
}

func TestCompoundSurvivesGroupsThatContainEachOther(t *testing.T) {
	a := layout.NewCNode(0, 0, 1)
	b := layout.NewCNode(0, 0, 0)
	a.IsGroup, b.IsGroup = true, true
	nodes := []layout.CNode{a, b, layout.NewCNode(40, 40, 0), layout.NewCNode(40, 40, -1)}
	edges := []layout.CEdge{edge(2, 3)}
	size := layout.CompoundLayout(context.Background(), nodes, edges, layout.TB, testOptions)
	if !finite(size.Width) {
		t.Errorf("width %v", size.Width)
	}
	if len(edges[0].Route) < 6 {
		t.Errorf("route %v", edges[0].Route)
	}
}
