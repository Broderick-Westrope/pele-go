package layout_test

import (
	"context"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/layout"
)

// Ported from upstream's tests/layout/lanes.test.ts.

func laneGroup(parent int) layout.CNode {
	node := layout.NewCNode(0, 0, parent)
	node.IsGroup = true
	node.PadX = 20
	node.PadTop = 36
	node.PadBottom = 20
	node.MinW = 60
	return node
}

type laneSample struct {
	nodes     []layout.CNode
	edges     []layout.CEdge
	lanes     []int
	leaves    []int
	laneOf    []int
	inner     []int
	innerLane []int
	dir       layout.Dir
}

func randomLanes(seed uint32) laneSample {
	rnd := random(seed)
	intn := func(n int) int { return int(math.Floor(rnd() * float64(n))) }
	var s laneSample
	leafCount := 1 + intn(30)
	laneCount := 1 + intn(5)
	// Leaves first, lanes after them, as the flowchart graph lists them.
	for i := 0; i < leafCount; i++ {
		s.nodes = append(s.nodes, layout.NewCNode(float64(30+intn(120)), float64(24+intn(60)), -1))
		s.leaves = append(s.leaves, i)
	}
	for k := 0; k < laneCount; k++ {
		s.lanes = append(s.lanes, len(s.nodes))
		s.nodes = append(s.nodes, laneGroup(-1))
	}
	// Upstream's loop bound, int(4), is drawn afresh on every pass.
	for k := 0; k < intn(4); k++ {
		lane := s.lanes[intn(laneCount)]
		s.inner = append(s.inner, len(s.nodes))
		s.innerLane = append(s.innerLane, lane)
		s.nodes = append(s.nodes, laneGroup(lane))
	}
	s.laneOf = make([]int, leafCount)
	for _, i := range s.leaves {
		if len(s.inner) > 0 && rnd() < 0.25 {
			g := intn(len(s.inner))
			s.nodes[i].Parent = s.inner[g]
			s.laneOf[i] = s.innerLane[g]
		} else {
			s.laneOf[i] = s.lanes[intn(laneCount)]
			s.nodes[i].Parent = s.laneOf[i]
		}
	}
	for k, count := 0, intn(leafCount*2); k < count; k++ {
		labelled := rnd() < 0.3
		src, dst := intn(leafCount), intn(leafCount)
		if labelled {
			s.edges = append(s.edges, edge(src, dst, float64(20+intn(80)), 18))
		} else {
			s.edges = append(s.edges, edge(src, dst))
		}
	}
	if rnd() < 0.3 {
		s.edges = append(s.edges, edge(s.lanes[0], s.leaves[0]))
	}
	s.dir = dirs[intn(4)]
	return s
}

func box(c layout.CNode) (float64, float64, float64, float64) {
	return c.X - c.W/2, c.Y - c.H/2, c.X + c.W/2, c.Y + c.H/2
}

// endSide is upstream's svg/shapes.ts endSide(): the side of c a route end is on (0 top, 1 right,
// 2 bottom, 3 left), or -1.
func endSide(route []float64, at int, c layout.CNode) int {
	dx := route[at] - c.X
	dy := route[at+1] - c.Y
	if math.Abs(math.Abs(dy)-c.H/2) < 0.5 {
		if dy < 0 {
			return 0
		}
		return 2
	}
	if math.Abs(math.Abs(dx)-c.W/2) < 0.5 {
		if dx < 0 {
			return 3
		}
		return 1
	}
	return -1
}

func TestLanesLaysOutNothing(t *testing.T) {
	if got := layout.LaneLayout(context.Background(), nil, nil, layout.TB, testOptions, layout.LanePad); got != (layout.CompoundResult{}) {
		t.Errorf("got %v", got)
	}
}

func TestLanesPutsLanesSideBySideInTheOrderWritten(t *testing.T) {
	for _, dir := range dirs {
		nodes := []layout.CNode{layout.NewCNode(60, 30, 3), layout.NewCNode(60, 30, 2), laneGroup(-1), laneGroup(-1)}
		result := layout.LaneLayout(context.Background(), nodes, []layout.CEdge{edge(0, 1)}, dir, testOptions, layout.LanePad)
		first, second := nodes[3], nodes[2]
		if dir == layout.TB || dir == layout.BT {
			if first.X >= second.X || first.H != result.Height || !closeTo(first.W+second.W, result.Width, 6) {
				t.Errorf("%s: lanes %v and %v in %v", dir, first, second, result)
			}
		} else if first.Y >= second.Y || first.W != result.Width || !closeTo(first.H+second.H, result.Height, 6) {
			t.Errorf("%s: lanes %v and %v in %v", dir, first, second, result)
		}
	}
}

func TestLanesRunTheFlowInTheDirectionAskedFor(t *testing.T) {
	at := func(dir layout.Dir) []layout.CNode {
		nodes := []layout.CNode{layout.NewCNode(60, 30, 2), layout.NewCNode(60, 30, 2), laneGroup(-1)}
		layout.LaneLayout(context.Background(), nodes, []layout.CEdge{edge(0, 1)}, dir, testOptions, layout.LanePad)
		return nodes
	}
	if n := at(layout.TB); n[0].Y >= n[1].Y {
		t.Error("TB does not run down")
	}
	if n := at(layout.BT); n[0].Y <= n[1].Y {
		t.Error("BT does not run up")
	}
	if n := at(layout.LR); n[0].X >= n[1].X {
		t.Error("LR does not run right")
	}
	if n := at(layout.RL); n[0].X <= n[1].X {
		t.Error("RL does not run left")
	}
}

func TestLanesKeepAChainInOneLaneOnAStraightLine(t *testing.T) {
	nodes := []layout.CNode{layout.NewCNode(50, 30, 3), layout.NewCNode(120, 30, 3), layout.NewCNode(70, 30, 3), laneGroup(-1)}
	layout.LaneLayout(context.Background(), nodes, []layout.CEdge{edge(0, 1), edge(1, 2)}, layout.TB, testOptions, layout.LanePad)
	if !closeTo(nodes[0].X, nodes[1].X, 6) || !closeTo(nodes[1].X, nodes[2].X, 6) {
		t.Errorf("chain is not straight: %v %v %v", nodes[0].X, nodes[1].X, nodes[2].X)
	}
}

func TestLanesSeparateADecisionsIncomingAcceptedAndReturnedPaths(t *testing.T) {
	for _, dir := range dirs {
		nodes := []layout.CNode{layout.NewCNode(60, 40, 8), layout.NewCNode(80, 40, 7), layout.NewCNode(70, 70, 7), layout.NewCNode(80, 40, 6), layout.NewCNode(80, 40, 6), layout.NewCNode(60, 40, 8), laneGroup(-1), laneGroup(-1), laneGroup(-1)}
		edges := []layout.CEdge{edge(0, 1, 70, 20), edge(1, 2), edge(2, 3, 50, 20), edge(3, 4), edge(4, 5), edge(2, 0, 70, 20)}
		layout.LaneLayout(context.Background(), nodes, edges, dir, testOptions, layout.LanePad)
		incoming := endSide(edges[1].Route, len(edges[1].Route)-3, nodes[2])
		accepted := endSide(edges[2].Route, 0, nodes[2])
		returned := endSide(edges[5].Route, 0, nodes[2])
		if incoming == accepted || accepted == returned || incoming == returned || slices.Contains([]int{incoming, accepted, returned}, -1) {
			t.Errorf("%s: sides %d %d %d", dir, incoming, accepted, returned)
		}
		back := edges[5]
		corners := 0
		for p := 6; p < len(back.Route); p += 3 {
			if (back.Route[p] == back.Route[p-3]) != (back.Route[p-3] == back.Route[p-6]) {
				corners++
			}
		}
		if corners != 1 {
			t.Errorf("%s: return path has %d corners", dir, corners)
		}
		for p := 3; p < len(back.Route); p += 3 {
			if back.Route[p] != back.Route[p-3] && back.Route[p+1] != back.Route[p-2] {
				t.Errorf("%s: return path has a slanted stretch", dir)
			}
		}
		for _, node := range nodes[:6] {
			l, tp, r, b := box(node)
			clear := back.LabelX+back.LabelW/2 <= l || back.LabelX-back.LabelW/2 >= r || back.LabelY+back.LabelH/2 <= tp || back.LabelY-back.LabelH/2 >= b
			if !clear {
				t.Errorf("%s: return label covers a node", dir)
			}
		}
	}
}

func TestLanesKeepAHandoffIntoAStepApartFromTheHandoffOut(t *testing.T) {
	for _, dir := range dirs {
		nodes := []layout.CNode{layout.NewCNode(110, 38, 5), layout.NewCNode(110, 38, 5), layout.NewCNode(100, 38, 4), layout.NewCNode(110, 38, 4), laneGroup(-1), laneGroup(-1)}
		edges := []layout.CEdge{edge(2, 3), edge(0, 3), edge(3, 1)}
		layout.LaneLayout(context.Background(), nodes, edges, dir, testOptions, layout.LanePad)
		into, out := edges[1].Route, edges[2].Route
		if endSide(into, len(into)-3, nodes[3]) == endSide(out, 0, nodes[3]) {
			t.Errorf("%s: handoffs in and out share a side", dir)
		}
		for p := 3; p < len(into); p += 3 {
			for q := 3; q < len(out); q += 3 {
				upright := into[p] == into[p-3] && out[q] == out[q-3] && into[p] == out[q]
				level := into[p+1] == into[p-2] && out[q+1] == out[q-2] && into[p+1] == out[q+1]
				k := 0
				if upright {
					k = 1
				}
				shared := min(max(into[p+k], into[p-3+k]), max(out[q+k], out[q-3+k])) - max(min(into[p+k], into[p-3+k]), min(out[q+k], out[q-3+k]))
				if (upright || level) && shared > 0 {
					t.Errorf("%s: handoffs run along one line for %v", dir, shared)
				}
			}
		}
	}
}

func TestLanesTakeAReturnPastTheStepsBetweenAroundTheMargin(t *testing.T) {
	for _, dir := range dirs {
		nodes := []layout.CNode{layout.NewCNode(140, 44, 8), layout.NewCNode(140, 44, 8), layout.NewCNode(140, 44, 7), layout.NewCNode(120, 76, 7), layout.NewCNode(140, 44, 6), layout.NewCNode(140, 44, 6), laneGroup(-1), laneGroup(-1), laneGroup(-1)}
		edges := []layout.CEdge{edge(0, 1), edge(1, 2), edge(2, 3), edge(3, 4, 30, 20), edge(4, 5), edge(3, 0, 30, 20)}
		layout.LaneLayout(context.Background(), nodes, edges, dir, testOptions, layout.LanePad)
		sideways := dir == layout.LR || dir == layout.RL
		across := func(c layout.CNode) float64 {
			if sideways {
				return c.Y
			}
			return c.X
		}
		along := func(c layout.CNode) float64 {
			if sideways {
				return c.X
			}
			return c.Y
		}
		depth := func(c layout.CNode) float64 {
			if sideways {
				return c.W
			}
			return c.H
		}
		if !closeTo(across(nodes[0]), across(nodes[1]), 6) || !closeTo(across(nodes[4]), across(nodes[5]), 6) {
			t.Errorf("%s: steps in a lane are not in line", dir)
		}
		back := edges[5].Route
		sides := []int{endSide(edges[2].Route, len(edges[2].Route)-3, nodes[3]), endSide(edges[3].Route, 0, nodes[3]), endSide(back, 0, nodes[3])}
		if sides[0] == sides[1] || sides[1] == sides[2] || sides[0] == sides[2] {
			t.Errorf("%s: sides %v", dir, sides)
		}
		if endSide(back, len(back)-3, nodes[0]) == endSide(edges[0].Route, 0, nodes[0]) {
			t.Errorf("%s: return arrives where the first step leaves", dir)
		}
		for p := 3; p < len(back); p += 3 {
			if back[p] != back[p-3] && back[p+1] != back[p-2] {
				t.Errorf("%s: return has a slanted stretch", dir)
			}
			for i, node := range nodes[:6] {
				if i == 0 || i == 3 {
					continue
				}
				l, tp, r, b := box(node)
				if !(max(back[p], back[p-3]) <= l || min(back[p], back[p-3]) >= r || max(back[p+1], back[p-2]) <= tp || min(back[p+1], back[p-2]) >= b) {
					t.Errorf("%s: return crosses node %d", dir, i)
				}
			}
		}
		gap := func(a, b int) float64 {
			return math.Abs(along(nodes[b])-along(nodes[a])) - (depth(nodes[a])+depth(nodes[b]))/2
		}
		if gap(1, 2) >= gap(0, 1) {
			t.Errorf("%s: ranks around a plain handoff do not close up", dir)
		}
		if !closeTo(gap(0, 1), testOptions.RankSep, 6) {
			t.Errorf("%s: gap %v", dir, gap(0, 1))
		}
	}
}

func TestLanesDrawALinkAskedToBeLongerAtItsLength(t *testing.T) {
	for _, dir := range dirs {
		nodes := []layout.CNode{layout.NewCNode(80, 40, 5), layout.NewCNode(80, 40, 5), layout.NewCNode(80, 40, 5), layout.NewCNode(80, 40, 4), laneGroup(-1), laneGroup(-1)}
		long := edge(0, 1)
		long.Minlen = 2
		edges := []layout.CEdge{long, edge(1, 2), edge(2, 3)}
		layout.LaneLayout(context.Background(), nodes, edges, dir, testOptions, layout.LanePad)
		sideways := dir == layout.LR || dir == layout.RL
		gap := func(a, b int) float64 {
			if sideways {
				return math.Abs(nodes[b].X-nodes[a].X) - 80
			}
			return math.Abs(nodes[b].Y-nodes[a].Y) - 40
		}
		if !closeTo(gap(0, 1), 2*testOptions.RankSep, 6) || !closeTo(gap(1, 2), testOptions.RankSep, 6) {
			t.Errorf("%s: gaps %v and %v", dir, gap(0, 1), gap(1, 2))
		}
	}
}

func TestLanesRouteAHandoffByWhereTheLabelOfARelationToALaneIs(t *testing.T) {
	nodes := []layout.CNode{layout.NewCNode(110, 38, 7), layout.NewCNode(110, 38, 6), layout.NewCNode(110, 38, 6), layout.NewCNode(110, 38, 6), layout.NewCNode(110, 38, 6), layout.NewCNode(110, 38, 6), laneGroup(-1), laneGroup(-1)}
	edges := []layout.CEdge{edge(0, 1), edge(1, 2), edge(2, 3), edge(3, 4), edge(4, 5), edge(6, 5, 400, 200)}
	layout.LaneLayout(context.Background(), nodes, edges, layout.TB, testOptions, layout.LanePad)
	handoff := edges[0].Route
	if len(handoff) <= 6 {
		t.Fatalf("handoff %v takes no corner", handoff)
	}
	for p := 3; p < len(handoff); p += 3 {
		if handoff[p] != handoff[p-3] && handoff[p+1] != handoff[p-2] {
			t.Error("handoff has a slanted stretch")
		}
	}
	for p := 1; p < len(handoff); p += 3 {
		if math.Abs(handoff[p]-edges[5].LabelY) <= 100 {
			t.Errorf("handoff passes %v from the label", math.Abs(handoff[p]-edges[5].LabelY))
		}
	}
}

func TestLanesKeepRandomGraphsWellFormed(t *testing.T) {
	inside := func(a, b layout.CNode) bool {
		l, tp, r, bt := box(a)
		ll, lt, lr, lb := box(b)
		return l >= ll-0.01 && r <= lr+0.01 && tp >= lt-0.01 && bt <= lb+0.01
	}
	for _, lanePad := range []float64{24, 12} {
		for seed := uint32(1); seed <= 300; seed++ {
			s := randomLanes(seed)
			result := layout.LaneLayout(context.Background(), s.nodes, s.edges, s.dir, testOptions, lanePad)
			where := fmt.Sprintf("pad %v seed %d", lanePad, seed)
			if !finite(result.Width, result.Height) {
				t.Fatalf("%s: size not finite", where)
			}
			for _, c := range s.nodes {
				if !finite(c.X, c.Y, c.W, c.H) {
					t.Errorf("%s: node not finite", where)
				}
			}
			for _, i := range s.leaves {
				if !inside(s.nodes[i], s.nodes[s.laneOf[i]]) {
					t.Errorf("%s: node %d outside its lane", where, i)
				}
			}
			for k, g := range s.inner {
				members := false
				for _, i := range s.leaves {
					if s.nodes[i].Parent == g {
						members = true
						if !inside(s.nodes[i], s.nodes[g]) {
							t.Errorf("%s: node %d outside its group", where, i)
						}
					}
				}
				if members && !inside(s.nodes[g], s.nodes[s.innerLane[k]]) {
					t.Errorf("%s: group %d outside its lane", where, g)
				}
			}
			for a := range s.leaves {
				for b := a + 1; b < len(s.leaves); b++ {
					al, at, ar, ab := box(s.nodes[s.leaves[a]])
					bl, bt, br, bb := box(s.nodes[s.leaves[b]])
					if !(ar <= bl+0.01 || br <= al+0.01 || ab <= bt+0.01 || bb <= at+0.01) {
						t.Errorf("%s: nodes %d and %d overlap", where, a, b)
					}
				}
			}
			for a := range s.lanes {
				l, tp, r, b := box(s.nodes[s.lanes[a]])
				if l < -0.01 || tp < -0.01 || r > result.Width+0.01 || b > result.Height+0.01 {
					t.Errorf("%s: lane outside the drawing", where)
				}
				for k := a + 1; k < len(s.lanes); k++ {
					ol, ot, or, ob := box(s.nodes[s.lanes[k]])
					if !(r <= ol+0.01 || or <= l+0.01 || b <= ot+0.01 || ob <= tp+0.01) {
						t.Errorf("%s: lanes overlap", where)
					}
				}
			}
			for _, e := range s.edges {
				if e.Src == e.Dst {
					continue
				}
				if len(e.Route) < 6 || len(e.Route)%3 != 0 || !finite(e.Route...) {
					t.Errorf("%s: route %v", where, e.Route)
					continue
				}
				for p := 0; p < len(e.Route); p += 3 {
					if e.Route[p] < -0.01 || e.Route[p] > result.Width+0.01 || e.Route[p+1] < -0.01 || e.Route[p+1] > result.Height+0.01 {
						t.Errorf("%s: route outside the lanes", where)
					}
				}
				if !finite(e.LabelX, e.LabelY) {
					t.Errorf("%s: label not finite", where)
				}
			}
		}
	}
}

func TestLanesGiveTheSameResultEveryTime(t *testing.T) {
	a := randomLanes(42)
	b := randomLanes(42)
	layout.LaneLayout(context.Background(), a.nodes, a.edges, a.dir, testOptions, layout.LanePad)
	layout.LaneLayout(context.Background(), b.nodes, b.edges, b.dir, testOptions, layout.LanePad)
	if !slices.Equal(a.nodes, b.nodes) {
		t.Error("nodes differ")
	}
	for i := range a.edges {
		if !slices.Equal(a.edges[i].Route, b.edges[i].Route) {
			t.Errorf("edge %d: routes differ", i)
		}
	}
}
