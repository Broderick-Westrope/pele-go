package class

import (
	"strconv"

	"github.com/Broderick-Westrope/pele-go/internal/omap"
)

type nodeShape int

const (
	shapeRect nodeShape = iota
	shapeClassBox
	shapeNote
	shapeInterface
)

// endMarker is upstream's EndMarker.
type endMarker string

const (
	markerAggregation endMarker = "aggregation"
	markerExtension   endMarker = "extension"
	markerComposition endMarker = "composition"
	markerDependency  endMarker = "dependency"
	markerLollipop    endMarker = "lollipop"
	markerNone        endMarker = "none"
)

type graphNode struct {
	id      string
	label   string
	isGroup bool
	shape   nodeShape
	// parentID "" is upstream's undefined.
	parentID string
	// Index of the namespace node this one is drawn in, or -1.
	parent    int
	classNode *classNode
}

type edgePattern int

const (
	patternSolid edgePattern = iota
	patternDashed
	patternDotted
)

type graphEdge struct {
	id    string
	start string
	end   string
	// Indexes into the node list. Ids alone are ambiguous: a class may be named `note0` or
	// `interface0`.
	source, target int
	// label "" is upstream's undefined.
	label           string
	arrowTypeStart  endMarker
	arrowTypeEnd    endMarker
	startLabelRight string
	endLabelLeft    string
	pattern         edgePattern
}

type classGraph struct {
	nodes []*graphNode
	edges []*graphEdge
}

var markers = [...]endMarker{markerAggregation, markerExtension, markerComposition, markerDependency, markerLollipop}

func marker(typ int) endMarker {
	if typ >= 0 && typ < len(markers) {
		return markers[typ]
	}
	return markerNone
}

// buildClassGraph resolves a parsed class diagram into the nodes and edges that are drawn.
// With hierarchical off, only namespaces the source declares are kept and each is labeled
// with its full name.
func buildClassGraph(model *classDb, hierarchical bool) classGraph {
	var nodes []*graphNode
	var edges []*graphEdge
	namespaceAt := map[string]int{}
	classAt := map[string]int{}
	interfaceAt := map[string]int{}

	// declared maps a namespace to its nearest declared ancestor, "" for none.
	var declared omap.Map[string, string]
	declaredAncestor := func(id string) string {
		if hierarchical || id == "" {
			return id
		}
		var chain []string
		current := id
		found := ""
		for current != "" {
			if v, ok := declared.Get(current); ok {
				found = v
				break
			}
			ns, ok := model.namespaces.Get(current)
			if !ok {
				break
			}
			if ns.explicit {
				found = current
				break
			}
			chain = append(chain, current)
			current = ns.parent
		}
		for _, visited := range chain {
			declared.Set(visited, found)
		}
		return found
	}
	group := func(id string) int {
		if id == "" {
			return -1
		}
		if at, ok := namespaceAt[id]; ok {
			return at
		}
		return -1
	}

	for _, ns := range model.namespaces.All() {
		if !hierarchical && !ns.explicit {
			continue
		}
		namespaceAt[ns.id] = len(nodes)
		node := &graphNode{id: ns.id, label: ns.id, isGroup: true, shape: shapeRect, parent: -1}
		if hierarchical {
			node.label, node.parentID = ns.label, ns.parent
		}
		nodes = append(nodes, node)
	}
	for _, node := range nodes {
		node.parent = group(node.parentID)
	}

	for _, cls := range model.classes.All() {
		parentID := declaredAncestor(cls.parent)
		classAt[cls.id] = len(nodes)
		nodes = append(nodes, &graphNode{
			id: cls.id, label: cls.label, shape: shapeClassBox,
			parentID: parentID, parent: group(parentID), classNode: cls,
		})
	}

	for _, n := range model.notes.All() {
		parentID := declaredAncestor(n.parent)
		if n.hasClass {
			if target, ok := classAt[n.class]; ok {
				edges = append(edges, &graphEdge{
					id:             "edgeNote" + strconv.Itoa(n.index),
					start:          n.id,
					end:            nodes[target].id,
					source:         len(nodes),
					target:         target,
					arrowTypeStart: markerNone,
					arrowTypeEnd:   markerNone,
					pattern:        patternDotted,
				})
			}
		}
		nodes = append(nodes, &graphNode{id: n.id, label: n.text, shape: shapeNote, parentID: parentID, parent: group(parentID)})
	}

	// Mermaid leaves every lollipop interface outside the namespaces. Here it stays beside
	// its class.
	for _, item := range model.interfaces {
		parent := -1
		if owner, ok := classAt[item.classID]; ok {
			parent = nodes[owner].parent
		}
		interfaceAt[item.id] = len(nodes)
		nodes = append(nodes, &graphNode{id: item.id, label: item.label, shape: shapeInterface, parent: parent})
	}

	lookup := func(at map[string]int, id string) int {
		if i, ok := at[id]; ok {
			return i
		}
		return -1
	}
	for count, r := range model.relations {
		// addRelation swaps in an interface for the lollipop end of a relation with no other
		// marker.
		startInterface := r.type1 == relationLollipop && r.type2 == relationNone
		endInterface := !startInterface && r.type2 == relationLollipop && r.type1 == relationNone
		startAt, endAt := classAt, classAt
		if startInterface {
			startAt = interfaceAt
		}
		if endInterface {
			endAt = interfaceAt
		}
		e := &graphEdge{
			id:             "id_" + r.id1 + "_" + r.id2 + "_" + strconv.Itoa(count+1),
			start:          r.id1,
			end:            r.id2,
			source:         lookup(startAt, r.id1),
			target:         lookup(endAt, r.id2),
			label:          r.title,
			arrowTypeStart: marker(r.type1),
			arrowTypeEnd:   marker(r.type2),
			pattern:        patternSolid,
		}
		if r.relationTitle1 != "none" {
			e.startLabelRight = r.relationTitle1
		}
		if r.relationTitle2 != "none" {
			e.endLabelLeft = r.relationTitle2
		}
		if r.lineType == 1 {
			e.pattern = patternDashed
		}
		edges = append(edges, e)
	}

	return classGraph{nodes: nodes, edges: edges}
}
