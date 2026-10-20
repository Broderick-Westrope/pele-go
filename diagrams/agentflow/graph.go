package agentflow

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// graphNode is a node as drawn. Empty strings stand for upstream's undefined, except where a
// pointer or flag says otherwise; colorIndex is -1 when unset.
type graphNode struct {
	id                string
	label             string
	labelType         labelKind
	shape             string
	isGroup           bool
	parentID          string
	hasParent         bool
	cssStyles         []string
	cssCompiledStyles []string
	cssClasses        string
	dir               string
	link              *string
	linkTarget        *string
	tooltip           *string
	// colorIndex is the palette slot of a container.
	colorIndex int
	// kind is what a node is, which picks its palette slot.
	kind     string
	metadata *jsobj.Object
}

// graphEdge is an edge as drawn. curve is the value upstream's || chain ends on, nil for
// undefined; minlen 0 is undefined.
type graphEdge struct {
	id                string
	isUserDefinedID   bool
	start             string
	end               string
	typ               string
	label             string
	labelType         labelKind
	thickness         string
	pattern           string
	minlen            float64
	arrowTypeStart    string
	arrowTypeEnd      string
	cssCompiledStyles []string
	style             []string
	classes           string
	animate           any
	animation         any
	curve             any
	metadata          *jsobj.Object
}

type agentGraph struct {
	nodes []*graphNode
	edges []*graphEdge
}

// buildAgentGraph resolves a parsed agentflow into the nodes and edges that are drawn: flow
// membership, collapsed flows, shapes reduced to the ones agentflow has, class styles and
// palette slots. Shape and containment problems are reported on the model as diagnostics.
// defaultCurve is nil when unset.
func buildAgentGraph(db *agentflowDb, paletteLength int, defaultCurve any) *agentGraph {
	graph := &agentGraph{}
	subgraphs := db.subgraphs

	compiled := func(classNames []string) []string {
		out := []string{}
		for _, name := range classNames {
			def, ok := db.classes.Get(name)
			if !ok {
				continue
			}
			for _, s := range def.styles {
				out = append(out, jsstr.Trim(s))
			}
			for _, s := range def.textStyles {
				out = append(out, jsstr.Trim(s))
			}
		}
		return out
	}
	isCollapsed := func(metadata *jsobj.Object) bool {
		v, _ := metadata.Get("view")
		return v == "collapsed"
	}

	// Everything inside a collapsed flow is hidden, and stands for its outermost collapsed
	// flow. Mermaid walks the whole inside of each collapsed flow, in the order the flows
	// closed, and the last walk to reach a thing wins. A thing has one container, so a later
	// walk can only enter an earlier one through its root: it stops there, and the root's
	// owner is followed afterwards.
	var hiddenOrder []string
	hidden := map[string]bool{}
	owner := map[string]string{}
	walked := map[string]bool{}
	for _, sg := range subgraphs {
		if !isCollapsed(sg.metadata) || hidden[sg.id] {
			continue
		}
		pending := []string{sg.id}
		for len(pending) > 0 {
			id := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			member := db.subgraph(id)
			if member == nil {
				continue
			}
			for _, childID := range member.nodes {
				if childID == sg.id || hidden[childID] {
					continue
				}
				hidden[childID] = true
				hiddenOrder = append(hiddenOrder, childID)
				owner[childID] = sg.id
				if !walked[childID] {
					pending = append(pending, childID)
				}
			}
		}
		walked[sg.id] = true
	}
	collapsedInto := map[string]string{}
	for _, id := range hiddenOrder {
		path := []string{id}
		top := owner[id]
		for {
			known, isKnown := collapsedInto[top]
			up, hasUp := known, isKnown
			if !isKnown {
				up, hasUp = owner[top]
			}
			if !hasUp {
				break
			}
			if !isKnown {
				path = append(path, top)
			}
			top = up
			if isKnown {
				break
			}
		}
		for _, p := range path {
			collapsedInto[p] = top
		}
	}

	position := map[string]int{}
	for i, sg := range subgraphs {
		position[sg.id] = i
	}

	// Two flows can each name the other. The nesting that would close the loop is dropped.
	parentOf := map[string]string{}
	closesLoop := func(childID, parentID string, at int) bool {
		// Only a flow handled earlier in this pass can already be above the parent.
		childAt, ok := position[childID]
		if !ok || childAt < at {
			return false
		}
		current, has := parentID, true
		for hops := 0; has && hops <= len(subgraphs); hops++ {
			if current == childID {
				return true
			}
			current, has = parentOf[current]
		}
		return false
	}
	for i := len(subgraphs) - 1; i >= 0; i-- {
		sg := subgraphs[i]
		if hidden[sg.id] {
			continue
		}
		for _, id := range sg.nodes {
			if closesLoop(id, sg.id, i) {
				db.emitWarning("CONTAINMENT_VIOLATION", `Container "`+sg.id+`" cannot contain "`+id+`" because "`+id+
					`" already contains it. The nesting that would close the loop is dropped.`, id)
				continue
			}
			parentOf[id] = sg.id
		}
	}

	byID := map[string]*graphNode{}
	for i := len(subgraphs) - 1; i >= 0; i-- {
		sg := subgraphs[i]
		if hidden[sg.id] {
			continue
		}
		collapsed := isCollapsed(sg.metadata)
		parent, hasParent := parentOf[sg.id]
		node := &graphNode{
			id:                sg.id,
			label:             sg.title,
			labelType:         sg.labelType,
			parentID:          parent,
			hasParent:         hasParent,
			cssStyles:         []string{},
			cssCompiledStyles: compiled(sg.classes),
			cssClasses:        strings.Join(sg.classes, " "),
			shape:             "flowGroup",
			dir:               sg.dir,
			isGroup:           !collapsed,
			colorIndex:        -1,
			metadata:          sg.metadata,
		}
		if collapsed {
			node.shape = "collapsedGroup"
			node.metadata = sg.metadata.Clone()
			node.metadata.Set("containerType", "flow")
		}
		graph.nodes = append(graph.nodes, node)
		byID[sg.id] = node
	}

	for _, vertex := range db.nodes.All() {
		if hidden[vertex.id] {
			continue
		}
		if existing := byID[vertex.id]; existing != nil {
			existing.cssStyles = vertex.styles
			existing.cssCompiledStyles = compiled(vertex.classes)
			existing.cssClasses = strings.Join(vertex.classes, " ")
			continue
		}
		shape := resolveShapeAlias(vertex.typ)
		if shape == "square" || shape == "" {
			shape = "squareRect"
		}
		parent, hasParent := parentOf[vertex.id]
		node := &graphNode{
			id:                vertex.id,
			label:             vertex.text,
			labelType:         vertex.labelType,
			parentID:          parent,
			hasParent:         hasParent,
			cssStyles:         vertex.styles,
			cssCompiledStyles: compiled(append([]string{"default", "node"}, vertex.classes...)),
			cssClasses:        "default " + strings.Join(vertex.classes, " "),
			link:              vertex.link,
			linkTarget:        vertex.linkTarget,
			metadata:          vertex.metadata,
			shape:             shape,
			colorIndex:        -1,
		}
		if tooltip, ok := db.tooltips[vertex.id]; ok {
			node.tooltip = &tooltip
		}
		graph.nodes = append(graph.nodes, node)
		byID[vertex.id] = node
	}

	for index, raw := range db.edges {
		start, ok := collapsedInto[raw.start]
		if !ok {
			start = raw.start
		}
		end, ok := collapsedInto[raw.end]
		if !ok {
			end = raw.end
		}
		// An edge between two things inside one collapsed flow has nothing left to connect.
		if start == end && raw.start != raw.end {
			continue
		}
		styles := append([]string{}, db.defaultEdgeStyle...)
		styles = append(styles, raw.style...)
		arrow := "arrow_point"
		switch raw.typ {
		case "arrow_open":
			arrow = "none"
		case "arrow_cross", "arrow_point":
			arrow = raw.typ
		}
		id := raw.id
		if id == "" {
			id = "L_" + start + "_" + end + "_" + strconv.Itoa(index)
		}
		typ := raw.typ
		if typ == "" {
			typ = "normal"
		}
		edge := &graphEdge{
			id:                id,
			isUserDefinedID:   raw.isUserDefinedID,
			start:             start,
			end:               end,
			typ:               typ,
			label:             raw.text,
			labelType:         raw.labelType,
			thickness:         raw.stroke,
			pattern:           raw.stroke,
			minlen:            raw.length,
			classes:           "edge-thickness-normal edge-pattern-solid flowchart-link",
			arrowTypeStart:    "none",
			arrowTypeEnd:      arrow,
			cssCompiledStyles: compiled(raw.classes),
			style:             styles,
			animate:           raw.animate,
			animation:         raw.animation,
			metadata:          raw.metadata,
		}
		// raw.interpolate || db.defaultInterpolate || defaultCurve
		switch {
		case jsobj.Truthy(raw.interpolate):
			edge.curve = raw.interpolate
		case db.defaultInterpolate != "":
			edge.curve = db.defaultInterpolate
		default:
			edge.curve = defaultCurve
		}
		graph.edges = append(graph.edges, edge)
	}

	normaliseNodeShapes(graph.nodes, db)

	// Containers are numbered in the order they were written: a walk of the nesting, parents
	// first. subgraphs itself is in the order the blocks closed, with a nested flow before
	// its parent.
	children := map[string][]string{}
	for _, sg := range subgraphs {
		if parent, ok := parentOf[sg.id]; ok {
			children[parent] = append(children[parent], sg.id)
		}
	}
	order := map[string]int{}
	for _, sg := range subgraphs {
		if _, ok := parentOf[sg.id]; ok {
			continue
		}
		pending := []string{sg.id}
		for len(pending) > 0 {
			id := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if _, done := order[id]; done {
				continue
			}
			order[id] = len(order)
			kids := children[id]
			for i := len(kids) - 1; i >= 0; i-- {
				pending = append(pending, kids[i])
			}
		}
	}

	assignColorSlots(graph.nodes, func(id string) string {
		if _, ok := db.connectors.Get(id); ok {
			return "connector"
		}
		if vertex, ok := db.nodes.Get(id); ok {
			return db.vertexKind(vertex)
		}
		return ""
	}, order, paletteLength)

	return graph
}
