package requirement

import (
	"strconv"
	"strings"
)

type graphNode struct {
	id          string
	requirement *requirement
	element     *requirementElement
	cssStyles   []string
	cssClasses  string
}

type graphEdge struct {
	id string
	// start and end are "" where upstream's are undefined: a node's name is never empty.
	start    string
	end      string
	typ      string
	contains bool
}

type requirementGraph struct {
	nodes     []graphNode
	edges     []graphEdge
	direction string
}

// buildRequirementGraph returns the nodes and edges to lay out: requirements first, then
// elements, as Mermaid orders them.
func buildRequirementGraph(db *requirementDb) requirementGraph {
	var nodes []graphNode
	for _, req := range db.requirements.All() {
		nodes = append(nodes, graphNode{
			id:          req.name,
			requirement: req,
			cssStyles:   req.cssStyles,
			cssClasses:  strings.Join(req.classes, " "),
		})
	}
	for _, el := range db.elements.All() {
		nodes = append(nodes, graphNode{
			id:         el.name,
			element:    el,
			cssStyles:  el.cssStyles,
			cssClasses: strings.Join(el.classes, " "),
		})
	}
	has := func(id string) string {
		if db.requirements.Has(id) || db.elements.Has(id) {
			return id
		}
		return ""
	}
	edges := make([]graphEdge, len(db.relations))
	for index, rel := range db.relations {
		edges[index] = graphEdge{
			id:       rel.src + "-" + rel.dst + "-" + strconv.Itoa(index),
			start:    has(rel.src),
			end:      has(rel.dst),
			typ:      rel.typ,
			contains: rel.typ == relContains,
		}
	}
	return requirementGraph{nodes: nodes, edges: edges, direction: db.direction}
}
