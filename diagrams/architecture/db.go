package architecture

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

// archNode is a service or, when junction is set, a junction. Empty strings stand for
// undefined.
type archNode struct {
	id                        string
	junction                  bool
	icon, iconText, title, in string
}

type archGroup struct {
	id, icon, title, in string
}

type archEdge = edgeAst

type archLayoutHint struct {
	direction string
	members   []string
}

func fail(message string) {
	e := perr.New(message, perr.Semantic)
	e.Type = "architecture"
	perr.Throw(e)
}

type architectureDb struct {
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
	nodes                     omap.Map[string, archNode]
	groups                    omap.Map[string, archGroup]
	edges                     []archEdge
	layoutHints               []archLayoutHint
	// registered says whether each id is a group (true) or a node.
	registered map[string]bool
}

func newArchitectureDb() *architectureDb {
	return &architectureDb{registered: map[string]bool{}}
}

func kindName(group bool) string {
	if group {
		return "group"
	}
	return "node"
}

func (db *architectureDb) register(kind, id, parent string) {
	if group, ok := db.registered[id]; ok {
		fail("The " + kind + " id [" + id + "] is already in use by another " + kindName(group))
	}
	if parent != "" {
		if id == parent {
			fail("The " + kind + " [" + id + "] cannot be placed within itself")
		}
		group, ok := db.registered[parent]
		if !ok {
			fail("The " + kind + " [" + id + "]'s parent does not exist. Please make sure the parent is created before this " + kind)
		}
		if !group {
			fail("The " + kind + " [" + id + "]'s parent is not a group")
		}
	}
	db.registered[id] = kind == "group"
}

func (db *architectureDb) addService(s serviceAst) {
	db.register("service", s.id, s.in)
	db.nodes.Set(s.id, archNode{id: s.id, icon: s.icon, iconText: s.iconText, title: s.title, in: s.in})
}

func (db *architectureDb) addJunction(j junctionAst) {
	db.register("junction", j.id, j.in)
	db.nodes.Set(j.id, archNode{id: j.id, junction: true, in: j.in})
}

func (db *architectureDb) addGroup(g groupAst) {
	db.register("group", g.id, g.in)
	db.groups.Set(g.id, archGroup{id: g.id, icon: g.icon, title: g.title, in: g.in})
}

func (db *architectureDb) addEdge(e archEdge) {
	type end struct {
		side, id string
		crosses  bool
	}
	ends := [2]end{{"left", e.lhsID, e.lhsGroup}, {"right", e.rhsID, e.rhsGroup}}
	for _, end := range ends {
		if _, ok := db.registered[end.id]; !ok {
			fail("The " + end.side + "-hand id [" + end.id + "] does not yet exist. Please create the service/group before declaring an edge to it.")
		}
	}
	// Mermaid lets a group id through the check above and then fails with a TypeError.
	for _, end := range ends {
		if !db.nodes.Has(end.id) {
			fail("The " + end.side + "-hand id [" + end.id + "] is a group. Add {group} to a service inside it instead.")
		}
	}
	lhs, _ := db.nodes.Get(e.lhsID)
	rhs, _ := db.nodes.Get(e.rhsID)
	if lhs.in != "" && lhs.in == rhs.in {
		for _, end := range ends {
			if end.crosses {
				fail("The " + end.side + "-hand id [" + end.id + "] is modified to traverse the group boundary, but the edge does not pass through two groups.")
			}
		}
	}
	db.edges = append(db.edges, e)
}

func (db *architectureDb) addLayoutHint(hint archLayoutHint) {
	if len(hint.members) < 2 {
		fail("An align directive requires at least two members; got " + strconv.Itoa(len(hint.members)))
	}
	seen := map[string]bool{}
	for _, id := range hint.members {
		if group, ok := db.registered[id]; !ok || group {
			fail("align " + hint.direction + " references [" + id + "], which is not a service or junction")
		}
		if seen[id] {
			fail("align " + hint.direction + " lists [" + id + "] more than once")
		}
		seen[id] = true
	}
	db.layoutHints = append(db.layoutHints, hint)
}

// populate is Mermaid's populateDb: every group, then services, junctions, edges, and
// alignments.
func populate(a architectureAst, db *architectureDb) {
	if a.accDescr != "" {
		db.accDescr = collapseIndentedBreaks(a.accDescr)
	}
	if a.accTitle != "" {
		db.accTitle = jsstr.TrimStart(a.accTitle)
	}
	if a.title != "" {
		db.title = a.title
	}
	for _, g := range a.groups {
		db.addGroup(g)
	}
	for _, s := range a.services {
		db.addService(s)
	}
	for _, j := range a.junctions {
		db.addJunction(j)
	}
	for _, e := range a.edges {
		db.addEdge(e)
	}
	for _, al := range a.alignments {
		db.addLayoutHint(archLayoutHint{direction: al.direction, members: append([]string(nil), al.members...)})
	}
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
		b.WriteByte('\n')
		i++
		for i < len(s) {
			r, n := jsstr.Decode(s, i)
			if !jsstr.IsSpace(r) {
				break
			}
			i += n
		}
	}
	return b.String()
}
