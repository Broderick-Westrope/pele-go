package class

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/text"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

// Mermaid's relationType numbers. A relation end that is not a marker is none.
const (
	relationAggregation = iota
	relationExtension
	relationComposition
	relationDependency
	relationLollipop
	relationNone = -1
)

// Mermaid looks up ids that start with a digit under this prefix, so they never match a class.
const domIDPrefix = "classId-"

// classNode is upstream's ClassNode. An empty parent, link, linkTarget or tooltip is
// upstream's undefined: none can otherwise be empty where it is read.
type classNode struct {
	id          string
	typ         string
	label       string
	cssClasses  string
	methods     []*classMember
	members     []*classMember
	annotations []string
	styles      []string
	parent      string
	link        string
	linkTarget  string
	tooltip     string
}

// relation is upstream's ClassRelation. An empty title is upstream's undefined.
type relation struct {
	id1, id2                       string
	relationTitle1, relationTitle2 string
	title                          string
	type1, type2                   int
	lineType                       int
}

type note struct {
	id string
	// class is the name the note is for; hasClass is false for a note for no class.
	class    string
	hasClass bool
	text     string
	index    int
	parent   string
}

type classInterface struct {
	id, label, classID string
}

// namespaceNode is upstream's NamespaceNode. An empty parent is upstream's undefined.
type namespaceNode struct {
	id       string
	label    string
	classes  omap.Map[string, *classNode]
	notes    omap.Map[string, *note]
	children omap.Map[string, *namespaceNode]
	parent   string
	// False for the ancestors created on the way to a dotted name such as `A.B.C`.
	explicit bool
}

type styleClass struct {
	id         string
	styles     []string
	textStyles []string
}

func splitName(id string) (className, typ string) {
	at := strings.IndexByte(id, '~')
	if at <= 0 {
		return id, ""
	}
	end := strings.IndexByte(id[at+1:], '~')
	if end == -1 {
		return id[:at], id[at+1:]
	}
	return id[:at], id[at+1 : at+1+end]
}

func lookupID(id string) string {
	if id != "" && id[0] >= '0' && id[0] <= '9' {
		id = domIDPrefix + id
	}
	className, _ := splitName(id)
	return className
}

// classDb is upstream's ClassDb, the model the parser builds and the renderer draws.
type classDb struct {
	direction    string
	classes      omap.Map[string, *classNode]
	relations    []*relation
	notes        omap.Map[string, *note]
	namespaces   omap.Map[string, *namespaceNode]
	interfaces   []classInterface
	styleClasses omap.Map[string, *styleClass]
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string

	namespaceStack []string
	withCSSClass   omap.Map[string, *omap.Set[*classNode]]
}

func newClassDb() *classDb {
	return &classDb{direction: "TB"}
}

func (db *classDb) addClass(id string) {
	className, typ := splitName(id)
	if db.classes.Has(className) {
		return
	}
	node := &classNode{id: className, typ: typ, label: className, cssClasses: "default"}
	db.classes.Set(className, node)
	db.tag(node, "default")
}

func (db *classDb) tag(node *classNode, cssClass string) {
	if tagged, ok := db.withCSSClass.Get(cssClass); ok {
		tagged.Add(node)
		return
	}
	tagged := &omap.Set[*classNode]{}
	tagged.Add(node)
	db.withCSSClass.Set(cssClass, tagged)
}

// ensure returns the class for a name that may carry a generic type, creating it if
// needed. Mermaid throws a TypeError where this creates, as for an annotation on a class
// not yet declared.
func (db *classDb) ensure(id string) *classNode {
	db.addClass(id)
	className, _ := splitName(id)
	node, _ := db.classes.Get(className)
	return node
}

func (db *classDb) setClassLabel(id, label string) {
	db.ensure(id).label = label
}

func (db *classDb) addRelation(r *relation) {
	switch {
	case r.type1 == relationLollipop && r.type2 == relationNone:
		db.addClass(r.id2)
		r.id1 = db.addInterface(r.id1, r.id2)
	case r.type2 == relationLollipop && r.type1 == relationNone:
		db.addClass(r.id1)
		r.id2 = db.addInterface(r.id2, r.id1)
	default:
		db.addClass(r.id1)
		db.addClass(r.id2)
	}
	r.id1, _ = splitName(r.id1)
	r.id2, _ = splitName(r.id2)
	r.relationTitle1 = jsstr.Trim(r.relationTitle1)
	r.relationTitle2 = jsstr.Trim(r.relationTitle2)
	db.relations = append(db.relations, r)
}

func (db *classDb) addInterface(label, classID string) string {
	id := "interface" + strconv.Itoa(len(db.interfaces))
	db.interfaces = append(db.interfaces, classInterface{id: id, label: label, classID: classID})
	return id
}

func (db *classDb) addAnnotation(className, annotation string) {
	node := db.ensure(className)
	node.annotations = append(node.annotations, annotation)
}

func (db *classDb) addMember(className, member string) {
	node := db.ensure(className)
	t := jsstr.Trim(member)
	switch {
	case strings.HasPrefix(t, "<<") && strings.HasSuffix(t, ">>"):
		node.annotations = append(node.annotations, t[2:len(t)-2])
	case strings.IndexByte(t, ')') > 0:
		parsed := newClassMember(t, true)
		if parsed.method {
			node.methods = append(node.methods, parsed)
		} else {
			node.members = append(node.members, parsed)
		}
	case t != "":
		node.members = append(node.members, newClassMember(t, false))
	}
}

// addMembers adds members given last to first, as the grammar collects them.
func (db *classDb) addMembers(className string, members []string) {
	for i := len(members) - 1; i >= 0; i-- {
		db.addMember(className, members[i])
	}
}

func (db *classDb) addNote(text string, className *string) string {
	index := db.notes.Len()
	id := "note" + strconv.Itoa(index)
	n := &note{id: id, text: text, index: index}
	if className != nil {
		n.class, n.hasClass = *className, true
	}
	db.notes.Set(id, n)
	return id
}

func cleanupLabel(label string) string {
	return jsstr.Trim(strings.TrimPrefix(label, ":"))
}

func (db *classDb) setCSSClass(ids, className string) {
	for _, id := range strings.Split(ids, ",") {
		node, ok := db.classes.Get(lookupID(id))
		if !ok {
			continue
		}
		node.cssClasses += " " + className
		for _, word := range strings.Split(className, " ") {
			db.tag(node, word)
		}
	}
}

func (db *classDb) defineClass(ids []string, style []string) {
	for _, id := range ids {
		sc, ok := db.styleClasses.Get(id)
		if !ok {
			sc = &styleClass{id: id}
			db.styleClasses.Set(id, sc)
		}
		for _, s := range style {
			if strings.Contains(s, "color") {
				sc.textStyles = append(sc.textStyles, strings.Replace(s, "fill", "bgFill", 1))
			}
			sc.styles = append(sc.styles, s)
		}
		// Mermaid tests `cssClasses.includes(id)`, which also matches part of a longer name
		// (a classDef named `def` styles every class, through `default`). Whole names are
		// compared here.
		tagged, ok := db.withCSSClass.Get(id)
		if !ok {
			continue
		}
		var parts []string
		for _, s := range style {
			parts = append(parts, strings.Split(s, ",")...)
		}
		for node := range tagged.All() {
			node.styles = append(node.styles, parts...)
		}
	}
}

func (db *classDb) setTooltip(ids, tooltip string) {
	for _, id := range strings.Split(ids, ",") {
		className, _ := splitName(id)
		if node, ok := db.classes.Get(className); ok {
			node.tooltip = tooltip
		}
	}
}

func (db *classDb) setLink(ids, link string, target *string) {
	// Entity codes are resolved first, so that what is checked is what a browser would follow.
	url := jsstr.Trim(text.DecodeEntities(link))
	for _, id := range strings.Split(ids, ",") {
		node, ok := db.classes.Get(lookupID(id))
		if !ok {
			continue
		}
		node.link = ""
		if url != "" {
			node.link = util.SanitizeURL(url)
		}
		node.linkTarget = "_blank"
		if target != nil {
			node.linkTarget = *target
		}
	}
	db.setCSSClass(ids, "clickable")
}

// setClickEvent marks the classes clickable. Callbacks are never executed, as in Mermaid's
// strict mode.
func (db *classDb) setClickEvent(ids string) {
	db.setCSSClass(ids, "clickable")
}

func (db *classDb) setDirection(dir string) {
	db.direction = dir
}

func (db *classDb) setAccTitle(s string) {
	db.accTitle = jsstr.TrimStart(s)
}

func (db *classDb) setAccDescription(s string) {
	db.accDescr = common.CollapseIndentedBreaks(s)
}

func (db *classDb) addNamespace(id, label string) string {
	prefix := ""
	if len(db.namespaceStack) > 0 {
		prefix = db.namespaceStack[len(db.namespaceStack)-1]
	}
	qualified := id
	if prefix != "" {
		qualified = prefix + "." + id
	}
	db.namespaceStack = append(db.namespaceStack, qualified)

	if existing, ok := db.namespaces.Get(qualified); ok {
		existing.explicit = true
		if label != "" {
			existing.label = label
		}
		return qualified
	}

	// The enclosing namespace and its ancestors exist already, so only the parts of `id`
	// are walked.
	parentID := prefix
	from := 0
	if prefix != "" {
		from = len(prefix) + 1
	}
	for {
		dot := strings.IndexByte(qualified[from:], '.')
		leaf := dot == -1
		if !leaf {
			dot += from
		}
		current := qualified
		if !leaf {
			current = qualified[:dot]
		}
		node, ok := db.namespaces.Get(current)
		if !ok {
			part := qualified[from:]
			if !leaf {
				part = qualified[from:dot]
			}
			node = &namespaceNode{id: current, label: part, parent: parentID, explicit: leaf}
			if leaf && label != "" {
				node.label = label
			}
			db.namespaces.Set(current, node)
		} else if leaf {
			node.explicit = true
		}
		if parentID != "" {
			if parent, ok := db.namespaces.Get(parentID); ok {
				if !parent.children.Has(current) {
					parent.children.Set(current, node)
				}
				if node.parent == "" {
					node.parent = parentID
				}
			}
		}
		if leaf {
			return qualified
		}
		parentID = current
		from = dot + 1
	}
}

func (db *classDb) popNamespace() {
	if len(db.namespaceStack) > 0 {
		db.namespaceStack = db.namespaceStack[:len(db.namespaceStack)-1]
	}
}

func (db *classDb) addClassesToNamespace(id string, classNames, noteNames []string) {
	ns, ok := db.namespaces.Get(id)
	if !ok {
		return
	}
	for _, name := range classNames {
		className, _ := splitName(name)
		node, ok := db.classes.Get(className)
		if !ok {
			continue
		}
		node.parent = id
		ns.classes.Set(className, node)
	}
	for _, name := range noteNames {
		n, ok := db.notes.Get(name)
		if !ok {
			continue
		}
		n.parent = id
		ns.notes.Set(name, n)
	}
}

func (db *classDb) setCSSStyle(id string, styles []string) {
	node, ok := db.classes.Get(id)
	if !ok {
		return
	}
	for _, s := range styles {
		node.styles = append(node.styles, strings.Split(s, ",")...)
	}
}
