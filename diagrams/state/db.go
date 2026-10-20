package state

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// stateDb is Mermaid's StateDB in its version 2 form, which is what Mermaid uses for both
// `stateDiagram` and `stateDiagram-v2`. The legacy version, which numbers `[*]` states, is
// not ported.
type stateDb struct {
	rootDoc   []stmt
	states    map[string]*stateStmt
	relations []stateRelation
	classes   map[string]*stateClassDef
	links     map[string]stateLink
	graph     stateGraph
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string

	dividerCount int
}

func newStateDb() *stateDb {
	return &stateDb{
		states:  map[string]*stateStmt{},
		classes: map[string]*stateClassDef{},
		links:   map[string]stateLink{},
	}
}

// direction is the direction the root document is laid out in: its last `direction` statement.
func (db *stateDb) direction() string {
	return docDirection(db.rootDoc)
}

func (db *stateDb) setRootDoc(doc []stmt) {
	db.rootDoc = doc
	db.translate()
	db.extract(doc)
}

// extract reads only the root document's statements into states, relations and classes; the
// statements inside composites are read when the graph is built.
func (db *stateDb) extract(doc []stmt) {
	db.states = map[string]*stateStmt{}
	db.relations = nil
	db.classes = map[string]*stateClassDef{}
	for _, item := range doc {
		switch item := item.(type) {
		case *stateStmt:
			db.addState(jsstr.Trim(item.id), item.typ, item.doc, item.description, item.note, nil)
		case *relationStmt:
			db.addRelation(item.state1, item.state2, item.description)
		case *classDefStmt:
			db.addStyleClass(jsstr.Trim(item.id), item.classes)
		case *styleStmt:
			parts := strings.Split(item.styleClass, ",")
			styles := make([]string, len(parts))
			for k, s := range parts {
				styles[k] = jsstr.Trim(strings.ReplaceAll(s, ";", ""))
			}
			for _, state := range db.named(jsstr.Trim(item.id)) {
				state.styles = append([]string{}, styles...)
			}
		case *applyClassStmt:
			db.setCssClass(jsstr.Trim(item.id), item.styleClass)
		case *clickStmt:
			db.addLink(item.id.id, item.url, item.tooltip)
		}
	}
	db.graph = buildStateGraph(doc, db.states, db.classes)
}

// pendingDoc is an entry of translate's work list.
type pendingDoc struct {
	parentID string
	node     stmt
	first    bool
}

// rootStmt is upstream's { stmt: 'root', id: 'root', doc }.
type rootStmt struct {
	id  string
	doc *[]stmt
}

func (*rootStmt) isStmt() {}

// translate is Mermaid's docTranslator: gives each `[*]` an id scoped to its parent, and
// turns a document split by `--` into one region per part.
func (db *stateDb) translate() {
	rootDoc := db.rootDoc
	pending := []pendingDoc{{"root", &rootStmt{id: "root", doc: &rootDoc}, true}}
	for len(pending) > 0 {
		top := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		var id string
		var docRef **[]stmt
		switch node := top.node.(type) {
		case *relationStmt:
			pending = append(pending, pendingDoc{top.parentID, node.state2, false}, pendingDoc{top.parentID, node.state1, true})
			continue
		case *stateStmt:
			if node.id == "[*]" {
				if top.first {
					node.id = top.parentID + "_start"
				} else {
					node.id = top.parentID + "_end"
				}
				start := top.first
				node.start = &start
			} else {
				node.id = jsstr.Trim(node.id)
			}
			id = node.id
			docRef = &node.doc
		case *rootStmt:
			id = node.id
			docRef = &node.doc
		default:
			continue
		}
		if *docRef == nil {
			continue
		}

		var regions []stmt
		current := []stmt{}
		for _, item := range **docRef {
			if s, ok := item.(*stateStmt); ok && s.typ == "divider" && s.doc == nil {
				region := *s
				doc := current
				region.doc = &doc
				regions = append(regions, &region)
				current = []stmt{}
			} else {
				current = append(current, item)
			}
		}
		// Mermaid names the region after the last divider with a random id; here it continues the
		// divider numbering. Mermaid also leaves a document that ends with a divider ungrouped and
		// then fails to draw it; here the regions before that divider are kept.
		if len(regions) > 0 {
			if len(current) > 0 {
				doc := current
				regions = append(regions, &stateStmt{id: db.getDividerID(), typ: "divider", doc: &doc})
			}
			*docRef = &regions
		}
		doc := **docRef
		for k := len(doc) - 1; k >= 0; k-- {
			pending = append(pending, pendingDoc{id, doc[k], true})
		}
	}
}

func (db *stateDb) addState(id, typ string, doc *[]stmt, description descr, note *stateNote, classes []string) {
	if typ == "" {
		typ = "default"
	}
	trimmedID := jsstr.Trim(id)
	state, ok := db.states[trimmedID]
	if !ok {
		state = &stateStmt{id: trimmedID, descriptions: []string{}, typ: typ, doc: doc, note: note, classes: []string{}, styles: []string{}, textStyles: []string{}}
		db.states[trimmedID] = state
	} else {
		if state.doc == nil {
			state.doc = doc
		}
		if state.typ == "" {
			state.typ = typ
		}
	}
	if description.truthy() {
		texts := description.list
		if !description.isList {
			texts = []string{description.s}
		}
		for _, text := range texts {
			db.addDescription(trimmedID, jsstr.Trim(text))
		}
	}
	if note != nil {
		state.note = note
	}
	for _, name := range classes {
		db.setCssClass(trimmedID, jsstr.Trim(name))
	}
}

func (db *stateDb) addLink(stateID, url, tooltip string) {
	db.links[stateID] = stateLink{url: url, tooltip: tooltip}
}

func (db *stateDb) addRelation(item1, item2 *stateStmt, title string) {
	for _, item := range [...]*stateStmt{item1, item2} {
		db.addState(item.id, item.typ, item.doc, item.description, item.note, item.classes)
	}
	db.relations = append(db.relations, stateRelation{id1: jsstr.Trim(item1.id), id2: jsstr.Trim(item2.id), relationTitle: title})
}

func (db *stateDb) addDescription(id, description string) {
	state, ok := db.states[id]
	if !ok {
		return
	}
	if rest, cut := strings.CutPrefix(description, ":"); cut {
		description = jsstr.Trim(rest)
	}
	state.descriptions = append(state.descriptions, description)
}

func (db *stateDb) getDividerID() string {
	db.dividerCount++
	return "divider-id-" + strconv.Itoa(db.dividerCount)
}

func (db *stateDb) addStyleClass(id, styleAttributes string) {
	found, ok := db.classes[id]
	if !ok {
		found = &stateClassDef{id: id, styles: []string{}, textStyles: []string{}}
		db.classes[id] = found
	}
	if styleAttributes == "" {
		return
	}
	for _, attrib := range strings.Split(styleAttributes, ",") {
		fixed := jsstr.Trim(strings.Replace(attrib, ";", "", 1))
		if strings.Contains(attrib, "color") {
			found.textStyles = append(found.textStyles, strings.Replace(strings.Replace(fixed, "fill", "bgFill", 1), "color", "fill", 1))
		}
		found.styles = append(found.styles, fixed)
	}
}

func (db *stateDb) setCssClass(itemIDs, cssClassName string) {
	for _, state := range db.named(itemIDs) {
		state.classes = append(state.classes, cssClassName)
	}
}

// setDirection is called by the grammar while it is still building the document, so the
// statement lands in the previous root document and the direction is read from the parsed
// `dir` statements instead.
func (db *stateDb) setDirection(dir string) {
	for _, item := range db.rootDoc {
		if d, ok := item.(*directionStmt); ok {
			d.value = dir
			return
		}
	}
	db.rootDoc = append([]stmt{&directionStmt{value: dir}}, db.rootDoc...)
}

func (db *stateDb) trimColon(text string) string {
	if rest, ok := strings.CutPrefix(text, ":"); ok {
		return jsstr.Trim(rest)
	}
	return jsstr.Trim(text)
}

func (db *stateDb) setAccTitle(text string) {
	db.accTitle = jsstr.TrimStart(text)
}

func (db *stateDb) setAccDescription(text string) {
	db.accDescr = collapseIndentedBreaks(text)
}

// collapseIndentedBreaks is s.replace(/\n\s+/g, '\n').
func collapseIndentedBreaks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\n' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		for j < len(s) {
			r, n := jsstr.Decode(s, j)
			if !jsstr.IsSpace(r) {
				break
			}
			j += n
		}
		b.WriteByte('\n')
		i = j
	}
	return b.String()
}

// named returns the states a comma-separated list of ids names, created where they do not
// exist yet. As in Mermaid, an id is looked up as written and trimmed only when it has to be
// created.
func (db *stateDb) named(ids string) []*stateStmt {
	var out []*stateStmt
	for _, id := range strings.Split(ids, ",") {
		state, ok := db.states[id]
		if !ok {
			db.addState(id, "", nil, descr{}, nil, nil)
			state, ok = db.states[jsstr.Trim(id)]
		}
		if ok {
			out = append(out, state)
		}
	}
	return out
}
