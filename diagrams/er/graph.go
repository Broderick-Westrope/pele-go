package er

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

type erGraphNode struct {
	id                string
	label             string
	alias             string
	attributes        []erAttribute
	isGroup           bool
	parentID          string
	hasParent         bool
	cssStyles         []string
	cssCompiledStyles []string
	cssClasses        string
	// dir is "" where upstream's is undefined.
	dir       string
	labelType string
}

type erGraphEdge struct {
	id             string
	start          string
	end            string
	label          string
	arrowTypeStart string
	arrowTypeEnd   string
	pattern        string
}

type erGraph struct {
	nodes []erGraphNode
	edges []erGraphEdge
}

// buildErGraph resolves a parsed ER diagram into what is drawn: subgraphs as groups, entities
// as nodes with their class styles, and relationships as edges with a marker type at each end.
func buildErGraph(model *erDb) erGraph {
	var nodes []erGraphNode
	var edges []erGraphEdge
	subgraphs := model.subgraphs

	compiled := func(classNames []string) []string {
		var out []string
		for _, name := range classNames {
			def, ok := model.classes.Get(name)
			if !ok {
				continue
			}
			for _, style := range def.styles {
				out = append(out, jsstr.Trim(style))
			}
			for _, style := range def.textStyles {
				out = append(out, jsstr.Trim(style))
			}
		}
		return out
	}

	parentOf := map[string]string{}
	subgraphIDs := map[string]bool{}
	for i := len(subgraphs) - 1; i >= 0; i-- {
		subgraph := subgraphs[i]
		subgraphIDs[subgraph.id] = true
		for _, id := range subgraph.nodes {
			parentOf[id] = subgraph.id
		}
	}

	for i := len(subgraphs) - 1; i >= 0; i-- {
		subgraph := subgraphs[i]
		parent, hasParent := parentOf[subgraph.id]
		nodes = append(nodes, erGraphNode{
			id:                subgraph.id,
			label:             subgraph.title,
			isGroup:           true,
			parentID:          parent,
			hasParent:         hasParent,
			cssStyles:         subgraph.cssStyles,
			cssCompiledStyles: compiled(subgraph.classes),
			cssClasses:        strings.Join(subgraph.classes, " "),
			dir:               subgraph.dir,
			labelType:         subgraph.labelType,
		})
	}

	// A name that is also a subgraph id stands for the subgraph. Mermaid drops such an entity
	// and leaves relationships written before the subgraph pointing at nothing; here they
	// follow it.
	moved := map[string]string{}
	for name, entity := range model.entities.All() {
		if subgraphIDs[name] {
			moved[entity.id] = name
			continue
		}
		parent, hasParent := parentOf[name]
		nodes = append(nodes, erGraphNode{
			id:                entity.id,
			label:             entity.label,
			alias:             entity.alias,
			attributes:        entity.attributes,
			parentID:          parent,
			hasParent:         hasParent,
			cssStyles:         entity.cssStyles,
			cssCompiledStyles: compiled(strings.Split(entity.cssClasses, " ")),
			cssClasses:        entity.cssClasses,
			labelType:         "markdown",
		})
	}

	for index, relationship := range model.relationships {
		start, ok := moved[relationship.entityA]
		if !ok {
			start = relationship.entityA
		}
		end, ok := moved[relationship.entityB]
		if !ok {
			end = relationship.entityB
		}
		pattern := "dashed"
		if relationship.relSpec.relType == "IDENTIFYING" {
			pattern = "solid"
		}
		edges = append(edges, erGraphEdge{
			id:             "id_" + relationship.entityA + "_" + relationship.entityB + "_" + strconv.Itoa(index),
			start:          start,
			end:            end,
			label:          relationship.roleA,
			arrowTypeStart: strings.ToLower(string(relationship.relSpec.cardB)),
			arrowTypeEnd:   strings.ToLower(string(relationship.relSpec.cardA)),
			pattern:        pattern,
		})
	}

	return erGraph{nodes: nodes, edges: edges}
}
