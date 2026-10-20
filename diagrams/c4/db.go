package c4

import (
	"math"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
)

// field returns the element field a name sets, or nil for one it has no field for. label
// is handled by named.
func (el *element) field(name string) *string {
	switch name {
	case "type":
		return &el.typ
	case "descr":
		return &el.descr
	case "techn":
		return &el.techn
	case "sprite":
		return &el.sprite
	case "tags":
		return &el.tags
	case "link":
		return &el.link
	case "bgColor":
		return &el.bgColor
	case "fontColor":
		return &el.fontColor
	case "borderColor":
		return &el.borderColor
	case "shadowing":
		return &el.shadowing
	case "shape":
		return &el.shape
	case "legendText":
		return &el.legendText
	case "legendSprite":
		return &el.legendSprite
	}
	return nil
}

func (r *rel) field(name string) *string {
	switch name {
	case "descr":
		return &r.descr
	case "techn":
		return &r.techn
	case "sprite":
		return &r.sprite
	case "tags":
		return &r.tags
	case "link":
		return &r.link
	case "textColor":
		return &r.textColor
	case "lineColor":
		return &r.lineColor
	}
	return nil
}

// target is an element or a relation.
type target interface {
	field(name string) *string
	setLabel(s string)
}

func (el *element) setLabel(s string) { el.label = s }
func (r *rel) setLabel(s string)      { r.label = s }

// named applies a named argument. Mermaid assigns one to whatever property it names,
// structural ones included. Here it reaches only the known fields; upstream keeps any other
// name aside in `extra`, which nothing reads, so it is dropped.
func applyNamed(t target, a *named) {
	if r, ok := t.(*rel); ok && (a.key == "offsetX" || a.key == "offsetY") {
		v := jsnum.ParseInt(a.value)
		if a.key == "offsetX" {
			r.offsetX = v
		} else {
			r.offsetY = v
		}
		return
	}
	if a.key == "label" {
		t.setLabel(a.value)
		return
	}
	if p := t.field(a.key); p != nil {
		*p = a.value
	}
}

func at(attrs []attr, i int) *attr {
	if i < len(attrs) {
		return &attrs[i]
	}
	return nil
}

// put fills the field of a positional argument's slot, or the field a named one names,
// whichever slot it is in.
func put(t target, field string, a *attr) {
	if a == nil {
		return
	}
	if a.n == nil {
		*t.field(field) = a.s
	} else {
		applyNamed(t, a.n)
	}
}

// putText is upstream's text(): an absent argument leaves the field as it is, or "" when it
// was undefined, which is the same here.
func putText(t target, field string, a *attr) {
	put(t, field, a)
}

func setLabel(t target, labelAttr **named, a *attr) {
	*labelAttr = nil
	switch {
	case a != nil && a.n != nil:
		t.setLabel("")
		*labelAttr = a.n
		applyNamed(t, a.n)
	case a != nil:
		t.setLabel(a.s)
	default:
		t.setLabel("")
	}
}

type relKey struct{ from, to string }

type c4Db struct {
	// c4Type is the header keyword: C4Context, C4Container, C4Component, C4Dynamic or
	// C4Deployment.
	c4Type   string
	title    string
	accDescr string
	shapes   []*element
	// The first boundary is the unnamed one that holds everything at the top level.
	boundaries    []*element
	rels          []*rel
	shapeInRow    float64
	boundaryInRow float64

	shapeIndex map[string]*element
	// The top-level boundary is not listed, so a boundary a diagram names `global` is its
	// own box. In Mermaid it replaces the top level and nothing is drawn.
	boundaryIndex map[string]*element
	relIndex      map[relKey]*rel
	current       *element
	stack         []*element
}

func newC4Db() *c4Db {
	global := &element{alias: "global", label: "global", typ: "global"}
	return &c4Db{
		boundaries:    []*element{global},
		shapeInRow:    4,
		boundaryInRow: 2,
		shapeIndex:    map[string]*element{},
		boundaryIndex: map[string]*element{},
		relIndex:      map[relKey]*rel{},
		current:       global,
	}
}

func (db *c4Db) setC4Type(typ string) { db.c4Type = typ }

func (db *c4Db) setTitle(title string) { db.title = title }

func (db *c4Db) setAccDescription(descr string) {
	db.accDescr = common.CollapseIndentedBreaks(descr)
}

func (db *c4Db) element(list *[]*element, index map[string]*element, alias attr, title *attr) *element {
	var el *element
	if alias.n == nil {
		el = index[alias.s]
	}
	if el == nil {
		el = &element{}
		*list = append(*list, el)
		if alias.n == nil {
			el.alias = alias.s
			index[alias.s] = el
		} else {
			el.aliasAttr = alias.n
			applyNamed(el, alias.n)
		}
	}
	setLabel(el, &el.labelAttr, title)
	return el
}

// alias, label, ?descr, ?sprite, ?tags, $link
func (db *c4Db) addPersonOrSystem(kind string, attrs []attr) {
	shape := db.element(&db.shapes, db.shapeIndex, attrs[0], at(attrs, 1))
	putText(shape, "descr", at(attrs, 2))
	db.finishShape(shape, kind, attrs, 3)
}

// alias, label, ?techn, ?descr, ?sprite, ?tags, $link
func (db *c4Db) addContainer(kind string, attrs []attr) {
	shape := db.element(&db.shapes, db.shapeIndex, attrs[0], at(attrs, 1))
	putText(shape, "techn", at(attrs, 2))
	putText(shape, "descr", at(attrs, 3))
	db.finishShape(shape, kind, attrs, 4)
}

func (db *c4Db) addComponent(kind string, attrs []attr) {
	db.addContainer(kind, attrs)
}

func (db *c4Db) finishShape(shape *element, kind string, attrs []attr, from int) {
	put(shape, "sprite", at(attrs, from))
	put(shape, "tags", at(attrs, from+1))
	put(shape, "link", at(attrs, from+2))
	shape.kind = kind
	shape.parent = db.current
}

func (db *c4Db) boundary(attrs []attr, fallback string) *element {
	b := db.element(&db.boundaries, db.boundaryIndex, attrs[0], at(attrs, 1))
	if a := at(attrs, 2); a == nil {
		b.typ = fallback
	} else {
		put(b, "type", a)
	}
	return b
}

func (db *c4Db) open(b *element) {
	b.parent = db.current
	db.stack = append(db.stack, db.current)
	db.current = b
}

// alias, label, ?type, ?tags, $link
func (db *c4Db) addPersonOrSystemBoundary(attrs []attr, fallback string) {
	b := db.boundary(attrs, fallback)
	put(b, "tags", at(attrs, 3))
	put(b, "link", at(attrs, 4))
	db.open(b)
}

func (db *c4Db) addContainerBoundary(attrs []attr) {
	db.addPersonOrSystemBoundary(attrs, "container")
}

// alias, label, ?type, ?descr, ?sprite, ?tags, $link
func (db *c4Db) addDeploymentNode(nodeType string, attrs []attr) {
	b := db.boundary(attrs, "node")
	putText(b, "descr", at(attrs, 3))
	// Mermaid drops whatever is in the sprite slot, a named argument included.
	put(b, "sprite", at(attrs, 4))
	put(b, "tags", at(attrs, 5))
	put(b, "link", at(attrs, 6))
	b.nodeType = nodeType
	db.open(b)
}

func (db *c4Db) popBoundaryParseStack() {
	if n := len(db.stack); n > 0 {
		db.current = db.stack[n-1]
		db.stack = db.stack[:n-1]
	} else {
		db.current = db.boundaries[0]
	}
}

// from, to, label, ?techn, ?descr, ?sprite, ?tags, $link
func (db *c4Db) addRel(typ string, attrs []attr) {
	if len(attrs) < 3 {
		return
	}
	from, to := attrs[0], attrs[1]
	known := from.n == nil && to.n == nil
	var r *rel
	if known {
		r = db.relIndex[relKey{from.s, to.s}]
	}
	if r == nil {
		r = &rel{typ: typ}
		db.rels = append(db.rels, r)
		if known {
			db.relIndex[relKey{from.s, to.s}] = r
		}
		if from.n == nil {
			r.from = from.s
		} else {
			r.fromAttr = from.n
		}
		if to.n == nil {
			r.to = to.s
		} else {
			r.toAttr = to.n
		}
	}
	r.typ = typ
	setLabel(r, &r.labelAttr, &attrs[2])
	putText(r, "techn", at(attrs, 3))
	putText(r, "descr", at(attrs, 4))
	put(r, "sprite", at(attrs, 5))
	put(r, "tags", at(attrs, 6))
	put(r, "link", at(attrs, 7))
}

func (db *c4Db) getC4Shape(alias string) *element {
	if el, ok := db.shapeIndex[alias]; ok {
		return el
	}
	return db.boundaryIndex[alias]
}

// elementName, ?bgColor, ?fontColor, ?borderColor, ?shadowing, ?shape, ?sprite, ?techn,
// ?legendText, ?legendSprite
func (db *c4Db) updateElStyle(attrs []attr) {
	if attrs[0].n != nil {
		return
	}
	el := db.getC4Shape(attrs[0].s)
	if el == nil {
		return
	}
	for i, f := range [...]string{"bgColor", "fontColor", "borderColor", "shadowing", "shape", "sprite", "techn", "legendText", "legendSprite"} {
		put(el, f, at(attrs, i+1))
	}
}

// from, to, ?textColor, ?lineColor, ?offsetX, ?offsetY
func (db *c4Db) updateRelStyle(attrs []attr) {
	from, to := at(attrs, 0), at(attrs, 1)
	if from == nil || to == nil || from.n != nil || to.n != nil {
		return
	}
	r := db.relIndex[relKey{from.s, to.s}]
	if r == nil {
		return
	}
	put(r, "textColor", at(attrs, 2))
	put(r, "lineColor", at(attrs, 3))
	if x := at(attrs, 4); x != nil {
		if x.n == nil {
			r.offsetX = jsnum.ParseInt(x.s)
		} else {
			applyNamed(r, x.n)
		}
	}
	if y := at(attrs, 5); y != nil {
		if y.n == nil {
			r.offsetY = jsnum.ParseInt(y.s)
		} else {
			applyNamed(r, y.n)
		}
	}
}

// ?c4ShapeInRow, ?c4BoundaryInRow. As in Mermaid, the slot decides which one a value sets,
// not its name.
func (db *c4Db) updateLayoutConfig(attrs []attr) {
	count := func(a *attr) float64 {
		switch {
		case a == nil:
			return math.NaN()
		case a.n == nil:
			return jsnum.ParseInt(a.s)
		default:
			return jsnum.ParseInt(a.n.value)
		}
	}
	shapes := count(at(attrs, 0))
	boundaries := count(at(attrs, 1))
	if shapes >= 1 {
		db.shapeInRow = shapes
	}
	if boundaries >= 1 {
		db.boundaryInRow = boundaries
	}
}
