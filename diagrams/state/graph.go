package state

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type stateNode struct {
	id string
	// rect, rectWithTitle, stateStart, stateEnd, choice, fork, join, divider, roundedWithTitle,
	// note or noteGroup; "" is undefined.
	shape             string
	label             string
	description       []string
	cssClasses        string
	cssCompiledStyles []string
	cssStyles         []string
	typ               string
	isGroup           bool
	parentId          *string
	// The direction inside a composite: its last `direction` statement, or TB. "" is undefined.
	dir        string
	position   *string
	noteFor    *string
	colorIndex *int
	// aside is set on the nodes made for a note, which have no colorIndex of their own.
	aside bool
}

// stateEdge has labels that are markdown throughout, as in Mermaid.
type stateEdge struct {
	id, start, end string
	label          string
	classes        string
	dashed         bool
}

type stateGraph struct {
	nodes []*stateNode
	edges []stateEdge
}

type draft struct {
	shape       string
	description descr
	cssClasses  string
	cssStyles   []string
	typ         string
	dir         string
	colorIndex  *int
}

type frame struct {
	item *stateStmt
	doc  []stmt
	at   int
	alt  bool
}

// length is description.length: the length of a string, or of a list.
func (d descr) length() int {
	if d.isList {
		return len(d.list)
	}
	return jsstr.Len(d.s)
}

// flat is [d].flat().
func (d descr) flat() []string {
	if d.isList {
		return append([]string{}, d.list...)
	}
	return []string{d.s}
}

func docDirection(doc []stmt) string {
	dir := "TB"
	for _, item := range doc {
		if d, ok := item.(*directionStmt); ok {
			dir = d.value
		}
	}
	return dir
}

// buildStateGraph is Mermaid's dataFetcher: walks the documents and resolves them into the
// nodes and edges that are drawn.
func buildStateGraph(rootDoc []stmt, states map[string]*stateStmt, classes map[string]*stateClassDef) stateGraph {
	var nodes []*stateNode
	var edges []stateEdge
	byID := map[string]*stateNode{}
	drafts := map[string]*draft{}
	slots := map[string]*int{}
	counter := 0
	nextSlot := 0

	insert := func(node *stateNode) {
		// The two literal ids are what DOMPurify leaves of `<<fork>>` and `<<choice>>` in Mermaid; kept for parity.
		if node.id == "" || node.id == "</join></fork>" || node.id == "</choice>" {
			return
		}
		if node.cssClasses != "" {
			// One copy, however many classes: a state named in a class statement many times has a long list.
			var compiled []string
			found := false
			for _, name := range strings.Split(node.cssClasses, " ") {
				def, ok := classes[name]
				if !ok {
					continue
				}
				if !found {
					compiled = append([]string{}, node.cssCompiledStyles...)
					found = true
				}
				compiled = append(compiled, def.styles...)
			}
			if found {
				node.cssCompiledStyles = compiled
			}
		}
		existing, ok := byID[node.id]
		if !ok {
			byID[node.id] = node
			nodes = append(nodes, node)
			return
		}
		// Object.assign(existing, node), for the keys each kind of node has.
		parentID := existing.parentId
		if node.parentId != nil {
			parentID = node.parentId
		}
		existing.shape = node.shape
		existing.label = node.label
		existing.description = node.description
		existing.cssClasses = node.cssClasses
		existing.cssCompiledStyles = node.cssCompiledStyles
		existing.cssStyles = node.cssStyles
		existing.typ = node.typ
		existing.isGroup = node.isGroup
		existing.dir = node.dir
		if node.aside {
			existing.position = node.position
		} else {
			existing.colorIndex = node.colorIndex
		}
		if node.noteFor != nil {
			existing.noteFor = node.noteFor
		}
		existing.parentId = parentID
	}

	visit := func(parent, item *stateStmt, alt bool) {
		id := item.id
		if id == "root" {
			return
		}
		known := states[id]
		classStr := ""
		styles := []string{}
		if known != nil {
			classStr = strings.Join(known.classes, " ")
			styles = known.styles
		}

		d, ok := drafts[id]
		if !ok {
			shape := "rect"
			if item.start != nil {
				if *item.start {
					shape = "stateStart"
				} else {
					shape = "stateEnd"
				}
			}
			if item.typ != "default" {
				shape = item.typ
			}
			d = &draft{
				shape:       shape,
				description: descr{s: id},
				cssClasses:  classStr + " statediagram-state",
				cssStyles:   styles,
			}
			drafts[id] = d
		}

		if item.description.truthy() {
			switch {
			case d.description.isList:
				d.shape = "rectWithTitle"
				d.description = descr{isList: true, list: append(append([]string{}, d.description.list...), item.description.flat()...)}
			case d.description.s != "":
				d.shape = "rectWithTitle"
				if d.description.s == id {
					d.description = descr{isList: true, list: item.description.flat()}
				} else {
					d.description = descr{isList: true, list: append([]string{d.description.s}, item.description.flat()...)}
				}
			default:
				d.shape = "rect"
				d.description = item.description
				if d.description.isList {
					d.description.list = item.description.flat()
				}
			}
		}
		if d.description.length() == 1 && d.shape == "rectWithTitle" {
			if d.typ == "group" {
				d.shape = "roundedWithTitle"
			} else {
				d.shape = "rect"
			}
		}

		if d.typ == "" && item.doc != nil {
			d.typ = "group"
			d.dir = docDirection(*item.doc)
			if item.typ == "divider" {
				d.shape = "divider"
			} else {
				d.shape = "roundedWithTitle"
			}
			// A container takes the next palette slot unless it is a region, which shares its composite's.
			shared, ok := (*int)(nil), false
			if parent != nil {
				shared, ok = slots[parent.id]
			}
			if d.shape == "divider" && ok {
				d.colorIndex = shared
			} else {
				slot := nextSlot
				nextSlot++
				if jsstr.Trim(classStr) != "" || len(styles) > 0 {
					d.colorIndex = nil
				} else {
					d.colorIndex = &slot
				}
			}
			slots[id] = d.colorIndex
			alternate := ""
			if alt {
				alternate = "statediagram-cluster-alt"
			}
			d.cssClasses = d.cssClasses + " statediagram-cluster " + alternate
		}

		node := &stateNode{
			id:                id,
			shape:             d.shape,
			cssClasses:        d.cssClasses,
			cssCompiledStyles: []string{},
			cssStyles:         d.cssStyles,
			typ:               d.typ,
			isGroup:           d.typ == "group",
			dir:               d.dir,
			colorIndex:        d.colorIndex,
		}
		if d.description.isList {
			node.label = d.description.list[0]
			node.description = append([]string{}, d.description.list[1:]...)
		} else {
			node.label = d.description.s
			node.description = []string{}
		}
		if node.shape == "divider" {
			node.label = ""
		}
		if parent != nil && parent.id != "root" {
			pid := parent.id
			node.parentId = &pid
		}

		note := item.note
		if note == nil {
			insert(node)
			return
		}
		// Mermaid puts each note in a group of its own, named after the state. Pele keeps that
		// group in the model and lays the note out beside its state instead.
		aside := func(asideID, shape, cssClasses string, isGroup bool) *stateNode {
			typ := "node"
			if isGroup {
				typ = "group"
			}
			position := note.position
			return &stateNode{
				id:                asideID,
				shape:             shape,
				label:             note.text,
				description:       []string{},
				cssClasses:        cssClasses,
				cssCompiledStyles: []string{},
				cssStyles:         []string{},
				typ:               typ,
				isGroup:           isGroup,
				position:          &position,
				aside:             true,
			}
		}
		noteNode := aside(id+"----note-"+strconv.Itoa(counter), "note", "statediagram-note", false)
		counter++
		groupID := id + "----parent"
		noteNode.parentId = &groupID
		noteFor := id
		noteNode.noteFor = &noteFor
		insert(aside(groupID, "noteGroup", d.cssClasses, true))
		insert(noteNode)
		insert(node)
		from, to := id, noteNode.id
		if note.position == "left of" {
			from, to = noteNode.id, id
		}
		edges = append(edges, stateEdge{id: from + "-" + to, start: from, end: to, classes: "transition note-edge", dashed: true})
	}

	stack := []*frame{{doc: rootDoc, alt: true}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		if f.at >= len(f.doc) {
			stack = stack[:len(stack)-1]
			continue
		}
		item := f.doc[f.at]
		f.at++
		switch item := item.(type) {
		case *stateStmt:
			visit(f.item, item, f.alt)
			if item.doc != nil {
				stack = append(stack, &frame{item: item, doc: *item.doc, alt: !f.alt})
			}
		case *relationStmt:
			visit(f.item, item.state1, f.alt)
			visit(f.item, item.state2, f.alt)
			edges = append(edges, stateEdge{id: "edge" + strconv.Itoa(counter), start: item.state1.id, end: item.state2.id, label: item.description, classes: "transition"})
			counter++
		}
	}

	for _, node := range nodes {
		if node.isGroup && len(node.description) > 0 {
			perr.Throw(&perr.Error{
				Code:    perr.Semantic,
				Message: "Group nodes can only have label. Remove the additional description for node [" + node.id + "]",
				Type:    "state",
			})
		}
	}
	return stateGraph{nodes: nodes, edges: edges}
}
