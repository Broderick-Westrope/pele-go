package layout

import (
	"context"
	"math"
	"slices"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
)

// Layout for swimlanes. Every top-level group is a lane: a band that runs the length of the
// diagram along the flow. Ranks are shared by all lanes, so a step sits level with the steps
// that happen at the same time in other lanes, and each node stays inside its own lane.

// LanePad is the room a lane keeps around its items unless the caller asks for another.
const LanePad = 24

const (
	laneHead      = 34
	laneSweeps    = 8
	laneDummies   = 20000
	maxHops       = 10000
	routeBudget   = 2000000
	laneNear      = 4 // Parallel lines closer than this are drawn as one.
	rounds        = 3
	relay         = 8
	turn          = 20
	laneMargin    = 20 // The room a lane keeps beside a handoff along its margin,
	laneLeg       = 14 // and how much of it is between the handoff and the lane's items.
	laneCloseRank = 3
)

type placement struct {
	width   float64
	height  float64
	routed  []uint8
	margins []uint8
	work    int
}

// LaneLayout lays out a swimlane diagram: every top-level group is a lane. lanePad is the room
// a lane keeps around its items; upstream's default is LanePad.
func LaneLayout(ctx context.Context, nodes []CNode, edges []CEdge, dir Dir, opt LayeredOptions, lanePad float64) CompoundResult {
	n := len(nodes)
	if n == 0 {
		return CompoundResult{}
	}
	sideways := dir == LR || dir == RL
	flipped := dir == BT || dir == RL

	// Lanes in the order they were written. The graph lists groups last-declared first.
	lanes := []int{}
	for i := n - 1; i >= 0; i-- {
		if nodes[i].Parent < 0 && nodes[i].IsGroup {
			lanes = append(lanes, i)
		}
	}
	laneCount := max(1, len(lanes))
	laneSlot := fillInt(n, -1)
	for k, id := range lanes {
		laneSlot[id] = k
	}
	hasChild := make([]uint8, n)
	for i := 0; i < n; i++ {
		if nodes[i].Parent >= 0 {
			hasChild[nodes[i].Parent] = 1
		}
	}
	// Room a lane keeps around its items, so that the groups drawn around them stay inside it.
	lanePads := make([]float64, laneCount)
	for i := range lanePads {
		lanePads[i] = lanePad
	}
	endPad := lanePad
	laneOf := func(i int) int {
		t := i
		room := lanePad
		for hops := 0; nodes[t].Parent >= 0 && hops < maxHops; hops++ {
			t = nodes[t].Parent
			if laneSlot[t] < 0 {
				room += max(nodes[t].PadX, nodes[t].PadTop, nodes[t].PadBottom)
			}
		}
		lane := max(0, laneSlot[t])
		if room > lanePads[lane] {
			lanePads[lane] = room
		}
		if room > endPad {
			endPad = room
		}
		return lane
	}

	// What gets ranked and placed: nodes, and groups that are empty and so act as nodes.
	itemOf := fillInt(n, -1)
	real := []int{}
	for i := 0; i < n; i++ {
		if laneSlot[i] >= 0 {
			continue
		}
		if !nodes[i].IsGroup || hasChild[i] == 0 {
			itemOf[i] = len(real)
			real = append(real, i)
		}
	}
	along := func(c *CNode) float64 {
		if sideways {
			return c.W
		}
		return c.H
	}
	cross := func(c *CNode) float64 {
		if sideways {
			return c.H
		}
		return c.W
	}

	lnodes := make([]LNode, len(real))
	for k, i := range real {
		lnodes[k] = NewLNode(cross(&nodes[i]), along(&nodes[i]), KindNode)
	}
	ledges := []LEdge{}
	edgeOf := []int{}
	step := 1
	{
		seen := map[int]struct{}{}
		for ei := range edges {
			e := &edges[ei]
			a := itemOf[e.Src]
			b := itemOf[e.Dst]
			if a < 0 || b < 0 || a == b {
				continue
			}
			key := min(a, b)*len(real) + max(a, b)
			if _, ok := seen[key]; e.LabelW > 0 || ok {
				step = 2
			}
			seen[key] = struct{}{}
			if sideways {
				ledges = append(ledges, NewLEdge(a, b, e.Minlen, e.LabelH, e.LabelW))
			} else {
				ledges = append(ledges, NewLEdge(a, b, e.Minlen, e.LabelW, e.LabelH))
			}
			edgeOf = append(edgeOf, ei)
		}
	}
	orient(ctx, lnodes, ledges)
	rank(ctx, lnodes, ledges, step)

	m := len(ledges)
	itemLane := make([]int, len(real))
	for k, i := range real {
		itemLane[k] = laneOf(i)
	}
	fstep := float64(step)
	// Lays the ranked items out and routes the relations. A plain relation is a handoff that takes
	// a simple route of its own, and so keeps no room through the ranks it passes.
	place := func(plain, margins []uint8) placement {
		pad := make([]float64, laneCount)
		for lane, room := range lanePads {
			if margins[lane] != 0 {
				pad[lane] = max(room, laneMargin)
			} else {
				pad[lane] = room
			}
		}
		used := make([]uint8, laneCount)
		// Items: the real ones, then a dummy for every rank an edge passes through.
		dummies := 0
		span := make([]int, m)
		for k := 0; k < m; k++ {
			span[k] = toInt32(math.Abs(float64(lnodes[ledges[k].Head].Rank - lnodes[ledges[k].Tail].Rank)))
			if plain[k] == 0 {
				dummies += span[k] - 1
			}
		}
		direct := dummies > laneDummies
		total := len(real)
		if !direct {
			total += dummies
		}
		RANK := make([]int, total)
		LANE := make([]int, total)
		SIZE := make([]float64, total)
		DEPTH := make([]float64, total)
		DUMMY := make([]uint8, total)
		maxRank := 0
		for k := range real {
			RANK[k] = lnodes[k].Rank
			LANE[k] = itemLane[k]
			SIZE[k] = lnodes[k].W
			DEPTH[k] = lnodes[k].H
			if RANK[k] > maxRank {
				maxRank = RANK[k]
			}
		}
		firstDummy := fillInt(m, -1)
		labelItem := fillInt(m, -1)
		links := []int{}
		{
			next := len(real)
			for k := 0; k < m; k++ {
				e := &ledges[k]
				a, b := e.Tail, e.Head
				if e.Reversed {
					a, b = e.Head, e.Tail
				}
				if direct || span[k] < 2 || plain[k] != 0 {
					links = append(links, a, b)
					continue
				}
				ra := RANK[a]
				rb := RANK[b]
				labelRank := -1
				if e.LabelW > 0 {
					labelRank = ra + ((rb - ra) >> 1)
					if step == 2 && labelRank&1 == 0 {
						labelRank--
					}
				}
				firstDummy[k] = next
				prev := a
				for r := ra + 1; r < rb; r++ {
					d := next
					next++
					RANK[d] = r
					// The edge crosses from its first lane to its last one evenly along the way.
					LANE[d] = LANE[a] + int(jsmath.Round(float64(float64(LANE[b]-LANE[a])*float64(r-ra))/float64(rb-ra)))
					DUMMY[d] = 1
					if r == labelRank {
						SIZE[d] = e.LabelW
						DEPTH[d] = e.LabelH
						labelItem[k] = d
					}
					links = append(links, prev, d)
					prev = d
				}
				links = append(links, prev, b)
			}
		}

		upStart := make([]int, total+1)
		downStart := make([]int, total+1)
		for k := 0; k < len(links); k += 2 {
			downStart[links[k]+1]++
			upStart[links[k+1]+1]++
		}
		for i := 0; i < total; i++ {
			upStart[i+1] += upStart[i]
			downStart[i+1] += downStart[i]
		}
		up := make([]int, len(links)/2)
		down := make([]int, len(links)/2)
		{
			uf := slices.Clone(upStart[:total])
			df := slices.Clone(downStart[:total])
			for k := 0; k < len(links); k += 2 {
				down[df[links[k]]] = links[k+1]
				df[links[k]]++
				up[uf[links[k+1]]] = links[k]
				uf[links[k+1]]++
			}
		}

		// Order within each rank: by lane first, then by where the item's neighbours sit.
		rows := make([][]int, maxRank+1)
		for i := 0; i < total; i++ {
			rows[RANK[i]] = append(rows[RANK[i]], i)
		}
		pos := make([]float64, total)
		key := make([]float64, total)
		settleRow := func(row []int) {
			slices.SortStableFunc(row, func(a, b int) int {
				if d := LANE[a] - LANE[b]; d != 0 {
					return d
				}
				if c := sign(key[a] - key[b]); c != 0 {
					return c
				}
				return a - b
			})
			for k, i := range row {
				pos[i] = float64(k)
			}
		}
		for _, row := range rows {
			for k, i := range row {
				key[i] = float64(k)
			}
			settleRow(row)
		}
		for sweep := 0; sweep < laneSweeps && maxRank > 0; sweep++ {
			checkCancel(ctx)
			downward := sweep&1 == 0
			for s := 1; s <= maxRank; s++ {
				row := rows[maxRank-s]
				start, list := downStart, down
				if downward {
					row = rows[s]
					start, list = upStart, up
				}
				for _, i := range row {
					sum := 0.0
					count := start[i+1] - start[i]
					for k := start[i]; k < start[i+1]; k++ {
						sum += pos[list[k]]
					}
					if count > 0 {
						key[i] = sum / float64(count)
					} else {
						key[i] = pos[i]
					}
				}
				settleRow(row)
			}
		}

		// The items of one rank that share a lane sit side by side as a run.
		gap := func(a, b int) float64 {
			if DUMMY[a] != 0 || DUMMY[b] != 0 {
				return opt.EdgeSep
			}
			return opt.NodeSep
		}
		eachRun := func(row []int, visit func(row []int, from, to int)) {
			from := 0
			for from < len(row) {
				to := from
				for to+1 < len(row) && LANE[row[to+1]] == LANE[row[from]] {
					to++
				}
				visit(row, from, to)
				from = to + 1
			}
		}
		runSize := func(row []int, from, to int) float64 {
			size := SIZE[row[from]]
			for k := from; k < to; k++ {
				size += gap(row[k], row[k+1]) + SIZE[row[k+1]]
			}
			return size
		}

		// Lane breadth: enough for its fullest rank, and for its title.
		breadth := make([]float64, laneCount)
		head := 0.0
		if len(lanes) > 0 {
			head = laneHead
		}
		for k := range lanes {
			lane := &nodes[lanes[k]]
			breadth[k] = max(0, lane.MinW-float64(2*pad[k]))
			if lane.PadTop > head {
				head = lane.PadTop
			}
		}
		for _, row := range rows {
			eachRun(row, func(_ []int, from, to int) {
				size := runSize(row, from, to)
				if size > breadth[LANE[row[from]]] {
					breadth[LANE[row[from]]] = size
				}
			})
		}
		laneStart := make([]float64, laneCount+1)
		for k := 0; k < laneCount; k++ {
			laneStart[k+1] = laneStart[k] + breadth[k] + float64(2*pad[k])
		}

		// Across the flow: each lane's items start centered, then line up with their neighbours in
		// the same lane as far as the lane and the items beside them allow.
		C := make([]float64, total)
		want := make([]float64, total)
		center := func(row []int, from, to int) {
			lane := LANE[row[from]]
			at := float64((laneStart[lane] + pad[lane] + (laneStart[lane+1] - pad[lane]) - runSize(row, from, to)) / 2)
			for k := from; k <= to; k++ {
				C[row[k]] = at + float64(SIZE[row[k]]/2)
				if k < to {
					at += SIZE[row[k]] + gap(row[k], row[k+1])
				}
			}
		}
		align := func(row []int, from, to int) {
			lane := LANE[row[from]]
			floor := laneStart[lane] + pad[lane]
			for k := from; k <= to; k++ {
				i := row[k]
				C[i] = max(want[i], floor+float64(SIZE[i]/2))
				if k < to {
					floor = C[i] + float64(SIZE[i]/2) + gap(i, row[k+1])
				}
			}
			ceil := laneStart[lane+1] - pad[lane]
			for k := to; k >= from; k-- {
				i := row[k]
				C[i] = min(C[i], ceil-float64(SIZE[i]/2))
				if k > from {
					ceil = C[i] - float64(SIZE[i]/2) - gap(row[k-1], i)
				}
			}
		}
		for _, row := range rows {
			eachRun(row, center)
		}
		for pass := 0; pass < 4 && maxRank > 0; pass++ {
			checkCancel(ctx)
			downward := pass&1 == 0
			for s := 1; s <= maxRank; s++ {
				row := rows[maxRank-s]
				start, list := downStart, down
				if downward {
					row = rows[s]
					start, list = upStart, up
				}
				for _, i := range row {
					sum := 0.0
					count := 0
					for k := start[i]; k < start[i+1]; k++ {
						if LANE[list[k]] != LANE[i] {
							continue
						}
						sum += C[list[k]]
						count++
					}
					if count > 0 {
						want[i] = sum / float64(count)
					} else {
						want[i] = C[i]
					}
				}
				eachRun(row, align)
			}
		}

		// Along the flow: one band per rank, after the lane titles.
		bandStart := make([]float64, maxRank+1)
		bandEnd := make([]float64, maxRank+1)
		// The room after a rank is for the relations that turn there or are only as long as they
		// were asked to be. Where just plain handoffs and straight lines pass, the ranks close up,
		// as far as the label of a handoff between them leaves it a line to sit on.
		sep := make([]float64, maxRank+1)
		for i := range sep {
			sep[i] = opt.RankSep / fstep
		}
		if slices.Contains(plain, 1) {
			full := opt.RankSep / fstep
			for i := range sep {
				sep[i] = full / laneCloseRank
			}
			for k := 0; k < m; k++ {
				e := &ledges[k]
				a, b := e.Tail, e.Head
				if e.Reversed {
					a, b = e.Head, e.Tail
				}
				if plain[k] != 0 {
					if e.LabelH <= 0 || span[k] > step {
						continue
					}
					room := min(full, (e.LabelH+2*turn-float64(min(DEPTH[a], DEPTH[b])/2))/float64(span[k]))
					for r := RANK[a]; r < RANK[b]; r++ {
						if room > sep[r] {
							sep[r] = room
						}
					}
					continue
				}
				d0 := firstDummy[k]
				if d0 < 0 || float64(span[k]) <= float64(fstep*e.Minlen) {
					for r := RANK[a]; r < RANK[b]; r++ {
						sep[r] = full
					}
					continue
				}
				for d := d0 - 1; d < d0+span[k]-1; d++ {
					u := d
					if d < d0 {
						u = a
					}
					v := b
					if d+1 < d0+span[k]-1 {
						v = d + 1
					}
					if LANE[u] != LANE[v] || math.Abs(C[u]-C[v]) >= 0.5 {
						sep[RANK[u]] = full
					}
				}
			}
		}
		at := head + endPad
		for r := 0; r <= maxRank; r++ {
			depth := 0.0
			for _, i := range rows[r] {
				if DEPTH[i] > depth {
					depth = DEPTH[i]
				}
			}
			bandStart[r] = at
			bandEnd[r] = at + depth
			if r == maxRank {
				at += depth + 0
			} else {
				at += depth + sep[r]
			}
		}
		length := at + endPad
		breadthTotal := laneStart[laneCount]
		flip := func(a float64) float64 {
			if flipped {
				return head + length - a
			}
			return a
		}
		px := func(c, a float64) float64 {
			if sideways {
				return flip(a)
			}
			return c
		}
		py := func(c, a float64) float64 {
			if sideways {
				return c
			}
			return flip(a)
		}
		axis := 0.0
		if sideways {
			axis = 1
		}

		for k := range real {
			node := &nodes[real[k]]
			a := float64((bandStart[RANK[k]] + bandEnd[RANK[k]]) / 2)
			node.X = px(C[k], a)
			node.Y = py(C[k], a)
		}
		for k := range lanes {
			lane := &nodes[lanes[k]]
			c := float64((laneStart[k] + laneStart[k+1]) / 2)
			size := laneStart[k+1] - laneStart[k]
			if sideways {
				lane.X, lane.Y, lane.W, lane.H = float64(length/2), c, length, size
			} else {
				lane.X, lane.Y, lane.W, lane.H = c, float64(length/2), size, length
			}
			lane.PadTop = head
		}

		// Groups inside a lane are drawn around their members, deepest first.
		depthOf := make([]int, n)
		inner := []int{}
		for i := 0; i < n; i++ {
			if !nodes[i].IsGroup || laneSlot[i] >= 0 || itemOf[i] >= 0 {
				continue
			}
			d := 0
			for p := nodes[i].Parent; p >= 0 && d < maxHops; p = nodes[p].Parent {
				d++
			}
			depthOf[i] = d
			inner = append(inner, i)
		}
		slices.SortStableFunc(inner, func(a, b int) int {
			if d := depthOf[b] - depthOf[a]; d != 0 {
				return d
			}
			return a - b
		})
		box := make([]float64, n*4)
		for i := 0; i < n; i++ {
			box[i*4] = math.Inf(1)
			box[i*4+1] = math.Inf(1)
			box[i*4+2] = math.Inf(-1)
			box[i*4+3] = math.Inf(-1)
		}
		grow := func(g int, c *CNode) {
			if c.X-float64(c.W/2) < box[g*4] {
				box[g*4] = c.X - float64(c.W/2)
			}
			if c.Y-float64(c.H/2) < box[g*4+1] {
				box[g*4+1] = c.Y - float64(c.H/2)
			}
			if c.X+float64(c.W/2) > box[g*4+2] {
				box[g*4+2] = c.X + float64(c.W/2)
			}
			if c.Y+float64(c.H/2) > box[g*4+3] {
				box[g*4+3] = c.Y + float64(c.H/2)
			}
		}
		for _, i := range real {
			if nodes[i].Parent >= 0 && laneSlot[nodes[i].Parent] < 0 {
				grow(nodes[i].Parent, &nodes[i])
			}
		}
		for _, g := range inner {
			c := &nodes[g]
			if math.IsInf(box[g*4], 1) {
				continue
			}
			w := max(box[g*4+2]-box[g*4]+float64(2*c.PadX), c.MinW)
			c.X = float64((box[g*4] + box[g*4+2]) / 2)
			c.W = w
			c.Y = float64((box[g*4+1] - c.PadTop + box[g*4+3] + c.PadBottom) / 2)
			c.H = box[g*4+3] - box[g*4+1] + c.PadTop + c.PadBottom
			if c.Parent >= 0 && laneSlot[c.Parent] < 0 {
				grow(c.Parent, c)
			}
		}

		for i := range edges {
			edges[i].Route = []float64{}
		}
		for k := 0; k < m; k++ {
			e := &ledges[k]
			out := &edges[edgeOf[k]]
			a, b := e.Tail, e.Head
			if e.Reversed {
				a, b = e.Head, e.Tail
			}
			pts := []float64{C[a], bandEnd[RANK[a]] - float64((bandEnd[RANK[a]]-bandStart[RANK[a]]-DEPTH[a])/2)}
			d0 := firstDummy[k]
			if d0 >= 0 {
				for d := d0; d < d0+span[k]-1; d++ {
					pts = append(pts, C[d], bandStart[RANK[d]])
					if bandEnd[RANK[d]] > bandStart[RANK[d]] {
						pts = append(pts, C[d], bandEnd[RANK[d]])
					}
				}
			}
			pts = append(pts, C[b], bandStart[RANK[b]]+float64((bandEnd[RANK[b]]-bandStart[RANK[b]]-DEPTH[b])/2))
			route := make([]float64, 0, len(pts)/2*3)
			for p := 0; p < len(pts); p += 2 {
				route = append(route, px(pts[p], pts[p+1]), py(pts[p], pts[p+1]), axis)
			}
			if e.Reversed {
				for i, j := 0, len(route)-3; i < j; i, j = i+3, j-3 {
					for t := 0; t < 2; t++ {
						route[i+t], route[j+t] = route[j+t], route[i+t]
					}
				}
			}
			out.Route = route
			label := labelItem[k]
			if label >= 0 {
				mid := float64((bandStart[RANK[label]] + bandEnd[RANK[label]]) / 2)
				out.LabelX = px(C[label], mid)
				out.LabelY = py(C[label], mid)
			} else {
				out.LabelX = float64((route[0] + route[len(route)-3]) / 2)
				out.LabelY = float64((route[1] + route[len(route)-2]) / 2)
			}
		}

		// An edge that starts or ends at a lane or a group runs straight to its border.
		for i := range edges {
			e := &edges[i]
			if len(e.Route) > 0 || e.Src == e.Dst {
				continue
			}
			a := nodes[e.Src]
			b := nodes[e.Dst]
			ax, ay := border(a, b)
			bx, by := border(b, a)
			flat := 0.0
			if math.Abs(bx-ax) > math.Abs(by-ay) {
				flat = 1
			}
			e.Route = []float64{ax, ay, flat, bx, by, flat}
			e.LabelX = float64((ax + bx) / 2)
			e.LabelY = float64((ay + by) / 2)
		}

		// A handoff can leave towards the other lane, then arrive along the flow (or vice versa).
		// In particular, a decision's return branch must not leave through its incoming side and
		// retrace the path past the preceding step. Try the two single-corner routes, keeping the
		// ranked route when a node or another label blocks them.
		routed := make([]uint8, m)
		routePoints := 0
		for i := range edges {
			routePoints += len(edges[i].Route) / 3
		}
		work := m * (len(real) + len(edges) + routePoints)
		if work <= routeBudget {
			amid := func(i int) float64 { return float64((bandStart[RANK[i]] + bandEnd[RANK[i]]) / 2) }
			labels := make([][2]float64, len(edges))
			for i := range edges {
				e := &edges[i]
				if sideways {
					labels[i] = [2]float64{e.LabelY, flip(e.LabelX)}
				} else {
					labels[i] = [2]float64{e.LabelX, flip(e.LabelY)}
				}
			}
			overlaps := func(c0, a0, c1, a1, lc, la, cw, ah float64) bool {
				return c1 > lc-float64(cw/2) && c0 < lc+float64(cw/2) && a1 > la-float64(ah/2) && a0 < la+float64(ah/2)
			}
			clearOf := func(c0, a0, c1, a1 float64, src, dst, ei int) bool {
				for i := range real {
					if i != src && i != dst && overlaps(c0, a0, c1, a1, C[i], amid(i), SIZE[i]+8, DEPTH[i]+8) {
						return false
					}
				}
				for i := range edges {
					e := &edges[i]
					cw, ah := e.LabelW, e.LabelH
					if sideways {
						cw, ah = e.LabelH, e.LabelW
					}
					// A step's own loop keeps its label inside the step's box, and has no place here.
					if i != ei && e.LabelW > 0 && e.Src != e.Dst && overlaps(c0, a0, c1, a1, labels[i][0], labels[i][1], cw, ah) {
						return false
					}
				}
				return true
			}
			// How many other relations have a stretch inside a box. A stretch's whole box counts,
			// including its curve.
			crossings := func(x0, y0, x1, y1 float64, ei int) int {
				count := 0
				for i := range edges {
					if i == ei {
						continue
					}
					route := edges[i].Route
					for j := 3; j < len(route); j += 3 {
						if max(route[j-3], route[j]) > x0 && min(route[j-3], route[j]) < x1 &&
							max(route[j-2], route[j+1]) > y0 && min(route[j-2], route[j+1]) < y1 {
							count++
							break
						}
					}
				}
				return count
			}
			meets := func(x, y float64, ei int) int {
				count := 0
				for i := range edges {
					route := edges[i].Route
					if i == ei || len(route) == 0 {
						continue
					}
					last := len(route) - 3
					if (math.Abs(route[0]-x) < 1 && math.Abs(route[1]-y) < 1) || (math.Abs(route[last]-x) < 1 && math.Abs(route[last+1]-y) < 1) {
						count++
					}
				}
				return count
			}
			// Two relations drawn along one line read as a single path. Those that leave the same step
			// or arrive at the same step may share a trunk; any other pair must not.
			retraces := func(x0, y0, x1, y1 float64, ei int) bool {
				upright := math.Abs(x1-x0) < 0.5
				lo, hi, line := min(x0, x1), max(x0, x1), y0
				along, across := 1, 0
				if upright {
					lo, hi, line = min(y0, y1), max(y0, y1), x0
					along, across = 0, 1
				}
				for i := range edges {
					other := &edges[i]
					if i == ei || other.Src == edges[ei].Src || other.Dst == edges[ei].Dst {
						continue
					}
					route := other.Route
					for j := 3; j < len(route); j += 3 {
						u0, u1 := route[j-3+along], route[j+along]
						if math.Abs(u0-line) >= laneNear || math.Abs(u1-line) >= laneNear {
							continue
						}
						v0, v1 := route[j-3+across], route[j+across]
						if min(hi, max(v0, v1))-max(lo, min(v0, v1)) > 1 {
							return true
						}
					}
				}
				return false
			}
			for k := 0; k < m; k++ {
				checkCancel(ctx)
				e, ei := &ledges[k], edgeOf[k]
				out := &edges[ei]
				a, b := e.Tail, e.Head
				if LANE[a] == LANE[b] || math.Abs(C[b]-C[a]) < 1 {
					continue
				}
				ac, bc, aa, ba := C[a], C[b], amid(a), amid(b)
				dc, da := mathSign(bc-ac), mathSign(ba-aa)
				attempt := func(pts []float64, margin float64) bool {
					for p := 2; p < len(pts); p += 2 {
						if !clearOf(min(pts[p-2], pts[p])-margin, min(pts[p-1], pts[p+1])-margin,
							max(pts[p-2], pts[p])+margin, max(pts[p-1], pts[p+1])+margin, a, b, ei) {
							return false
						}
						if retraces(px(pts[p-2], pts[p-1]), py(pts[p-2], pts[p-1]), px(pts[p], pts[p+1]), py(pts[p], pts[p+1]), ei) {
							return false
						}
					}
					lc, la, longest := 0.0, 0.0, -1.0
					if out.LabelW > 0 {
						cw, ah := e.LabelW, e.LabelH
						for p := 2; p < len(pts); p += 2 {
							cc, ca := float64((pts[p-2]+pts[p])/2), float64((pts[p-1]+pts[p+1])/2)
							across, along := math.Abs(pts[p]-pts[p-2]), math.Abs(pts[p+1]-pts[p-1])
							run := across + along
							short := along < ah+8
							if across > 0 {
								short = across < cw+8
							}
							if run <= longest || short {
								continue
							}
							if cc-float64(cw/2) < 0 || cc+float64(cw/2) > breadthTotal || ca-float64(ah/2) < 0 || ca+float64(ah/2) > length {
								continue
							}
							// Include the ends here: the label must clear both shapes it connects.
							if !clearOf(cc-float64(cw/2), ca-float64(ah/2), cc+float64(cw/2), ca+float64(ah/2), -1, -1, ei) {
								continue
							}
							// A label moved onto a handoff must not cover a return path that keeps its
							// ranked route. A segment's whole box is kept clear, including its curve.
							lx, ly := px(cc, ca), py(cc, ca)
							if crossings(lx-float64(out.LabelW/2), ly-float64(out.LabelH/2), lx+float64(out.LabelW/2), ly+float64(out.LabelH/2), ei) > 0 {
								continue
							}
							lc, la, longest = cc, ca, run
						}
						if longest < 0 {
							return false
						}
					}
					// A point short of each corner and one past it keep the turn close to the corner, so
					// that the line stays under its label and the handoff takes little room.
					way := []float64{pts[0], pts[1]}
					for p := 2; p < len(pts); p += 2 {
						dx, dy := pts[p]-pts[p-2], pts[p+1]-pts[p-1]
						run := math.Abs(dx) + math.Abs(dy)
						first, last := 0.0, 0.0
						if p > 2 {
							first = turn
						}
						if p+2 < len(pts) {
							last = turn
						}
						if run > first+last+1 {
							if first != 0 {
								way = append(way, pts[p-2]+float64((dx/run)*first), pts[p-1]+float64((dy/run)*first))
							}
							if last != 0 {
								way = append(way, pts[p]-float64((dx/run)*last), pts[p+1]-float64((dy/run)*last))
							}
						}
						way = append(way, pts[p], pts[p+1])
					}
					out.Route = []float64{}
					for p := 0; p < len(way); p += 2 {
						var crosswise bool
						if p == 0 {
							crosswise = way[0] != way[2]
						} else {
							crosswise = way[p] != way[p-2]
						}
						ax := axis
						if crosswise {
							ax = 1 - axis
						}
						out.Route = append(out.Route, px(way[p], way[p+1]), py(way[p], way[p+1]), ax)
					}
					if out.LabelW > 0 {
						out.LabelX = px(lc, la)
						out.LabelY = py(lc, la)
						labels[ei] = [2]float64{lc, la}
					}
					routed[k] = 1
					return true
				}
				if attempt([]float64{ac + float64(float64(dc*SIZE[a])/2), aa, bc, aa, bc, ba - float64(float64(da*DEPTH[b])/2)}, 2) {
					continue
				}
				if attempt([]float64{ac, aa + float64(float64(da*DEPTH[a])/2), ac, ba, bc - float64(float64(dc*SIZE[b])/2), ba}, 2) {
					continue
				}
				// With both corners blocked, as for a return past the steps between, the handoff goes
				// around: out to the margin of a lane, along it, and back in level with its end. It
				// takes the margin that crosses the fewest other relations and shares the fewest ends
				// with them, then the shortest way.
				if work > routeBudget/relay {
					continue
				}
				type way struct {
					pts  []float64
					cost float64
					lane int
				}
				around := []way{}
				pair := [2]int{LANE[a], LANE[b]}
				if LANE[a] < LANE[b] {
					pair = [2]int{LANE[b], LANE[a]}
				}
				for _, lane := range pair {
					// A lane that keeps no margin yet has less room there, and the handoff stays inside it.
					inset := max(pad[lane]-laneLeg, min(float64(pad[lane]/2), laneMargin-laneLeg))
					for _, ch := range [2]float64{laneStart[lane] + inset, laneStart[lane+1] - inset} {
						if math.Abs(ch-ac) < float64(SIZE[a]/2)+4 || math.Abs(ch-bc) < float64(SIZE[b]/2)+4 {
							continue
						}
						pts := []float64{ac + float64(float64(mathSign(ch-ac)*SIZE[a])/2), aa, ch, aa, ch, ba, bc + float64(float64(mathSign(ch-bc)*SIZE[b])/2), ba}
						cost := float64(meets(px(pts[0], pts[1]), py(pts[0], pts[1]), ei)+meets(px(pts[6], pts[7]), py(pts[6], pts[7]), ei)) * 1e6
						for p := 2; p < len(pts); p += 2 {
							x0, y0, x1, y1 := px(pts[p-2], pts[p-1]), py(pts[p-2], pts[p-1]), px(pts[p], pts[p+1]), py(pts[p], pts[p+1])
							cost += float64(float64(crossings(min(x0, x1), min(y0, y1), max(x0, x1), max(y0, y1), ei))*1e6) + math.Abs(x1-x0) + math.Abs(y1-y0)
						}
						around = append(around, way{pts, cost, lane})
					}
				}
				slices.SortStableFunc(around, func(p, q way) int { return sign(p.cost - q.cost) })
				for _, w := range around {
					if !attempt(w.pts, 1) {
						continue
					}
					used[w.lane] = 1
					break
				}
			}
		}

		width, height := breadthTotal, length
		if sideways {
			width, height = length, breadthTotal
		}
		return placement{width: width, height: height, routed: routed, margins: used, work: work}
	}

	plain := make([]uint8, m)
	margins := make([]uint8, laneCount)
	placed := place(plain, margins)
	// A handoff that was given a simple route needs no room kept for it through the ranks it
	// passes, so the lanes are laid out again without it. A handoff that then loses its route
	// goes back to the ranked one. Routing is most of the work, so a large graph is laid out once.
	for round := 0; round < rounds && placed.work <= routeBudget/relay; round++ {
		checkCancel(ctx)
		next := placed.routed
		if round > 0 {
			next = make([]uint8, m)
			for k, on := range plain {
				next[k] = on & placed.routed[k]
			}
		}
		if slices.Equal(next, plain) && slices.Equal(placed.margins, margins) {
			break
		}
		plain = next
		margins = placed.margins
		placed = place(plain, margins)
	}
	for k, on := range plain {
		if on != 0 && placed.routed[k] == 0 {
			placed = place(make([]uint8, m), make([]uint8, laneCount))
			break
		}
	}
	return CompoundResult{Width: placed.width, Height: placed.height}
}

// mathSign is Math.sign: it keeps -0 and NaN.
func mathSign(v float64) float64 {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return v
}

// border is the point where the line from one box's center to another's leaves the first box.
// When the other center is inside the box, the point is on the nearest side instead.
func border(from, to CNode) (float64, float64) {
	dx := to.X - from.X
	dy := to.Y - from.Y
	hw := max(float64(from.W/2), 0.5)
	hh := max(float64(from.H/2), 0.5)
	if math.Abs(dx) < hw && math.Abs(dy) < hh {
		toSide := hw - math.Abs(dx)
		toEnd := hh - math.Abs(dy)
		if toSide < toEnd {
			if dx < 0 {
				return from.X - hw, to.Y
			}
			return from.X + hw, to.Y
		}
		if dy < 0 {
			return to.X, from.Y - hh
		}
		return to.X, from.Y + hh
	}
	sx, sy := math.Inf(1), math.Inf(1)
	if dx != 0 {
		sx = hw / math.Abs(dx)
	}
	if dy != 0 {
		sy = hh / math.Abs(dy)
	}
	scale := min(sx, sy)
	return from.X + float64(dx*scale), from.Y + float64(dy*scale)
}
