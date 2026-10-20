package mindmap

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type mindmapNode struct {
	id     int
	nodeID string
	level  int
	descr  string
	typ    int
	// children are the node's children, in source order.
	children []*mindmapNode
	isRoot   bool
	// section is the index of the first-level branch the node belongs to, wrapping after
	// eleven as in Mermaid. The root's is upstream's undefined, which every use reads as 0.
	section int
	class   string
	icon    *string
}

const sections = 11

type mindmapDb struct {
	title *string
	// nodes are in source order, which is depth-first. The first node is the root.
	nodes []*mindmapNode

	baseLevel int
	// open is the chain of nodes that can still take children, with strictly increasing levels.
	open []*mindmapNode
}

func (db *mindmapDb) GetType(start, end string) int { return common.GetType(start, end) }

func (db *mindmapDb) AddNode(level int, id, descr string, typ int, _ *string) {
	isRoot := len(db.nodes) == 0
	if isRoot {
		db.baseLevel = level
	}
	level -= db.baseLevel

	// Mermaid's parent is the last node with a smaller indentation, wherever it is in the tree.
	depth := len(db.open)
	for depth > 0 && db.open[depth-1].level >= level {
		depth--
	}
	var parent *mindmapNode
	if depth > 0 {
		parent = db.open[depth-1]
	}
	if parent == nil && !isRoot {
		e := perr.New(`There can be only one root. No parent could be found for ("`+descr+`")`, perr.Semantic)
		e.Type = "mindmap"
		perr.Throw(e)
	}
	node := &mindmapNode{id: len(db.nodes), nodeID: id, level: level, descr: descr, typ: typ, isRoot: isRoot}
	if parent != nil {
		if parent.isRoot {
			node.section = len(parent.children) % sections
		} else {
			node.section = parent.section
		}
		parent.children = append(parent.children, node)
	}
	db.open = append(db.open[:depth], node)
	db.nodes = append(db.nodes, node)
}

// DecorateNode ignores a decoration before any node, where Mermaid throws a TypeError.
func (db *mindmapDb) DecorateNode(d common.Decoration) {
	if len(db.nodes) == 0 {
		return
	}
	node := db.nodes[len(db.nodes)-1]
	if d.Icon != nil && *d.Icon != "" {
		node.icon = d.Icon
	}
	if d.Class != nil && *d.Class != "" {
		node.class = *d.Class
	}
}
