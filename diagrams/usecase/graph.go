package usecase

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
)

type jsonRow struct {
	// Blank on the second and later values of an array.
	key string
	// The key again, for every row.
	accessibleKey string
	value         string
}

// graphNode is upstream's UsecaseGraphNode. Its optional strings are "" where upstream's are
// undefined.
type graphNode struct {
	id        string
	label     string
	labelType string
	// Mermaid's shape names.
	shape             string
	isGroup           bool
	padding           float64
	cssClasses        string
	cssStyles         []string
	cssCompiledStyles []string
	actorType         string
	business          bool
	icon              string
	stereotype        string
	parentID          string
	noteTarget        string
	// isJSON is whether jsonRows is set.
	isJSON       bool
	jsonRows     []jsonRow
	boundaryType string
}

type graphEdge struct {
	id                string
	start             string
	end               string
	relationshipType  string
	pattern           string
	arrowTypeStart    string
	arrowTypeEnd      string
	label             string
	labelType         string
	classes           string
	style             []string
	cssCompiledStyles []string
	animate           bool
	animation         string
	minlen            int
	explicitID        bool
	// The dotted line from a note to what it is about.
	internal bool
}

type usecaseGraph struct {
	nodes []*graphNode
	edges []*graphEdge
}

var actorShapes = map[string]string{
	"normal":  "usecaseActor",
	"hollow":  "usecaseActorHollow",
	"awesome": "usecaseActorAwesome",
	"icon":    "usecaseActorIcon",
}

// markerTypes are the start and end markers for each arrow type, in the order of the arrow
// types.
var markerTypes = [...][2]string{
	{"none", "arrow_point"},
	{"arrow_point", "none"},
	{"none", "none"},
	{"none", "arrow_circle"},
	{"none", "arrow_cross"},
	{"arrow_circle", "none"},
	{"arrow_cross", "none"},
}

// undefined is a key JSON.parse did not give the object, which reads as undefined.
type undefined struct{}

func scalar(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return "null"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return jsnum.Format(v)
	}
	return "undefined"
}

func isObject(v any) bool {
	switch v.(type) {
	case *jsobj.Object, []any:
		return true
	}
	return false
}

// flattenJSONRows makes one row for each leaf of a JSON object, depth first in the order the
// keys were written. Nesting is kept on a stack of its own, so depth is limited by memory and
// not by the call stack.
func flattenJSONRows(value *jsobj.Object, propertyOrder map[string][]string) []jsonRow {
	type item struct {
		value   any
		path    string
		pointer string
	}
	rows := []jsonRow{}
	stack := []item{{value, "", ""}}
	var children []item
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		path := current.path
		children = children[:0]
		switch v := current.value.(type) {
		case []any:
			if len(v) == 0 {
				rows = append(rows, jsonRow{key: path, accessibleKey: path, value: "[]"})
				break
			}
			scalars := true
			for _, e := range v {
				if isObject(e) {
					scalars = false
					break
				}
			}
			for i, e := range v {
				if scalars {
					key := ""
					if i == 0 {
						key = path
					}
					rows = append(rows, jsonRow{key: key, accessibleKey: path, value: scalar(e)})
				} else {
					index := jsnum.Format(float64(i))
					children = append(children, item{e, path + "[" + index + "]", current.pointer + "/" + index})
				}
			}
		case *jsobj.Object:
			keys, ok := propertyOrder[current.pointer]
			if !ok {
				keys = v.Keys()
			}
			if len(keys) == 0 {
				rows = append(rows, jsonRow{key: path, accessibleKey: path, value: "{}"})
			}
			for _, key := range keys {
				var child any = undefined{}
				if e, has := v.Get(key); has {
					child = e
				}
				childPath := key
				if path != "" {
					childPath = path + "." + key
				}
				children = append(children, item{child, childPath, current.pointer + "/" + pointerKey(key)})
			}
		default:
			rows = append(rows, jsonRow{key: path, accessibleKey: path, value: scalar(v)})
		}
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
	return rows
}

// classList joins the names that are not empty.
func classList(names ...string) string {
	var kept []string
	for _, n := range names {
		if n != "" {
			kept = append(kept, n)
		}
	}
	return strings.Join(kept, " ")
}

func markers(r *relationship) [2]string {
	if r.typ == "generalization" {
		return [2]string{"none", "extension"}
	}
	if r.typ != "association" {
		return markerTypes[arrowSolidArrow]
	}
	if r.arrowType >= 0 && r.arrowType < len(markerTypes) {
		return markerTypes[r.arrowType]
	}
	return markerTypes[arrowLineSolid]
}

func when(ok bool, s string) string {
	if ok {
		return s
	}
	return ""
}

// buildUsecaseGraph resolves a parsed use case diagram into what is drawn: every actor, use
// case, note, JSON table and boundary as a node with its class styles, and every relationship
// and note connector as an edge with its markers. Ports getData from Mermaid's use case DB.
func buildUsecaseGraph(model *usecaseModel) usecaseGraph {
	var g usecaseGraph

	// compiled: a later class wins a property but keeps the place the property first took.
	compiled := func(classNames []string) []string {
		var out omap.Map[string, string]
		for _, name := range append([]string{"default"}, classNames...) {
			def, ok := model.classDefs.Get(name)
			if !ok {
				continue
			}
			for _, raw := range def.styles {
				style := jsstr.Trim(raw)
				property := style
				if colon := strings.IndexByte(style, ':'); colon != -1 {
					property = style[:colon]
				}
				property = jsstr.Trim(property)
				if property != "" {
					out.Set(property, style)
				}
			}
		}
		list := []string{}
		for _, v := range out.All() {
			list = append(list, v)
		}
		return list
	}
	clone := func(s []string) []string { return append([]string{}, s...) }

	for _, a := range model.actors.All() {
		g.nodes = append(g.nodes, &graphNode{
			id:        a.id,
			label:     a.label,
			labelType: a.labelType,
			shape:     actorShapes[a.typ],
			padding:   10,
			cssClasses: classList(append([]string{"default", "usecase-actor", "usecase-actor-" + a.typ,
				when(a.business, "usecase-business")}, a.classes...)...),
			cssStyles:         clone(a.styles),
			cssCompiledStyles: compiled(a.classes),
			actorType:         a.typ,
			business:          a.business,
			icon:              a.icon,
			stereotype:        a.stereotype,
			parentID:          a.parentID,
		})
	}
	for _, u := range model.useCases.All() {
		ellipse := u.shape == "ellipse"
		shape := "usecaseEllipse"
		switch {
		case !ellipse:
			shape = "rect"
		case u.business:
			shape = "usecaseBusiness"
		}
		padding := 10.0
		if ellipse {
			padding = 20
		}
		g.nodes = append(g.nodes, &graphNode{
			id:        u.id,
			label:     u.label,
			labelType: u.labelType,
			shape:     shape,
			padding:   padding,
			cssClasses: classList(append([]string{"default", "usecase-element", "usecase-" + u.shape,
				when(u.business, "usecase-business")}, u.classes...)...),
			cssStyles:         clone(u.styles),
			cssCompiledStyles: compiled(u.classes),
			business:          u.business,
			stereotype:        u.stereotype,
			parentID:          u.parentID,
		})
	}
	for _, note := range model.notes.All() {
		g.nodes = append(g.nodes, &graphNode{
			id:                note.id,
			label:             note.label,
			labelType:         note.labelType,
			shape:             "note",
			padding:           10,
			cssClasses:        "default usecase-note",
			cssStyles:         []string{},
			cssCompiledStyles: compiled(nil),
			noteTarget:        note.target,
		})
	}
	for _, j := range model.jsonNodes.All() {
		g.nodes = append(g.nodes, &graphNode{
			id:                j.id,
			label:             j.id,
			labelType:         "text",
			shape:             "usecaseJsonTable",
			padding:           10,
			cssClasses:        classList(append([]string{"default", "usecase-json-table"}, j.classes...)...),
			cssStyles:         clone(j.styles),
			cssCompiledStyles: compiled(j.classes),
			isJSON:            true,
			jsonRows:          flattenJSONRows(j.value, j.propertyOrder),
		})
	}
	for _, b := range model.systemBoundaries.All() {
		g.nodes = append(g.nodes, &graphNode{
			id:        b.id,
			label:     b.label,
			labelType: b.labelType,
			shape:     "usecaseSystemBoundary",
			isGroup:   true,
			padding:   20,
			cssClasses: classList(append([]string{"default", "system-boundary", "system-boundary-" + b.typ},
				b.classes...)...),
			cssStyles:         clone(b.styles),
			cssCompiledStyles: compiled(b.classes),
			boundaryType:      b.typ,
		})
	}

	for _, r := range model.relationships {
		semantic := r.typ == "include" || r.typ == "extend"
		m := markers(r)
		animated := r.animate || r.animation != ""
		animation := r.animation
		if animation == "" {
			animation = "fast"
		}
		pattern := "solid"
		if semantic {
			pattern = "dotted"
		}
		classes := append([]string{"default", "relationship", "relationship-" + r.typ}, r.classes...)
		edge := &graphEdge{
			id:                r.id,
			start:             r.source,
			end:               r.target,
			relationshipType:  r.typ,
			pattern:           pattern,
			arrowTypeStart:    m[0],
			arrowTypeEnd:      m[1],
			classes:           classList(append(classes, when(animated, "edge-animation-"+animation))...),
			style:             clone(r.styles),
			cssCompiledStyles: compiled(r.classes),
			animate:           r.animate,
			animation:         r.animation,
			minlen:            r.minlen,
			explicitID:        r.explicitID,
		}
		// Include and extend are labelled with their kind, whatever label the relationship carries.
		if semantic {
			edge.label = r.typ
			edge.labelType = "text"
		} else if r.typ == "association" {
			edge.label = r.label
			edge.labelType = r.labelType
		}
		g.edges = append(g.edges, edge)
	}
	for _, note := range model.notes.All() {
		g.edges = append(g.edges, &graphEdge{
			id:                note.id + "-edge",
			start:             note.id,
			end:               note.target,
			relationshipType:  "note",
			pattern:           "dotted",
			arrowTypeStart:    "none",
			arrowTypeEnd:      "none",
			classes:           "default relationship relationship-note",
			style:             []string{},
			cssCompiledStyles: compiled(nil),
			minlen:            1,
			internal:          true,
		})
	}
	return g
}
