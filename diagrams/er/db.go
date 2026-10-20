package er

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
)

func labelType(typ string) string {
	if typ == "markdown" || typ == "string" || typ == "text" {
		return typ
	}
	return "markdown"
}

// erDb is upstream's ErDb, which is also the model the renderer reads.
type erDb struct {
	direction     string
	entities      omap.Map[string, *erEntity]
	relationships []erRelationship
	classes       omap.Map[string, *erClassDef]
	subgraphs     []*erSubgraph
	// title, accTitle and accDescr are "" where upstream's are undefined: every use treats
	// the two alike.
	title          string
	accTitle       string
	accDescr       string
	subgraphLookup map[string]*erSubgraph
	claimed        map[string]bool
}

func newErDb() *erDb {
	return &erDb{direction: "TB", subgraphLookup: map[string]*erSubgraph{}, claimed: map[string]bool{}}
}

func (db *erDb) addEntity(name, alias string) *erEntity {
	entity, ok := db.entities.Get(name)
	if !ok {
		entity = &erEntity{
			id:         "entity-" + name + "-" + strconv.Itoa(db.entities.Len()),
			label:      name,
			alias:      alias,
			cssClasses: "default",
		}
		db.entities.Set(name, entity)
	} else if entity.alias == "" && alias != "" {
		entity.alias = alias
	}
	return entity
}

// addAttributes: the grammar builds the list back to front, so the last attribute written
// comes first.
func (db *erDb) addAttributes(name string, attributes []erAttribute) {
	entity := db.addEntity(name, "")
	for i := len(attributes) - 1; i >= 0; i-- {
		entity.attributes = append(entity.attributes, attributes[i])
	}
}

func (db *erDb) addRelationship(entA, roleA, entB string, spec relSpec) {
	entityA := entA
	if _, ok := db.subgraphLookup[entA]; !ok {
		entityA = db.addEntity(entA, "").id
	}
	entityB := entB
	if _, ok := db.subgraphLookup[entB]; !ok {
		entityB = db.addEntity(entB, "").id
	}
	db.relationships = append(db.relationships, erRelationship{entityA: entityA, roleA: roleA, entityB: entityB, relSpec: spec})
}

func (db *erDb) setDirection(dir string) {
	db.direction = dir
}

func (db *erDb) addCssStyles(ids, styles []string) {
	for _, id := range ids {
		if entity, ok := db.entities.Get(id); ok {
			entity.cssStyles = append(entity.cssStyles, styles...)
		}
		if subgraph, ok := db.subgraphLookup[id]; ok {
			subgraph.cssStyles = append(subgraph.cssStyles, styles...)
		}
	}
}

func (db *erDb) addClass(ids, styles []string) {
	for _, id := range ids {
		def, ok := db.classes.Get(id)
		if !ok {
			def = &erClassDef{id: id}
			db.classes.Set(id, def)
		}
		for _, style := range styles {
			if strings.Contains(style, "color") {
				def.textStyles = append(def.textStyles, strings.Replace(style, "fill", "bgFill", 1))
			}
			def.styles = append(def.styles, style)
		}
	}
}

func (db *erDb) setClass(ids, classNames []string) {
	for _, id := range ids {
		if entity, ok := db.entities.Get(id); ok {
			for _, name := range classNames {
				entity.cssClasses += " " + name
			}
		}
		if subgraph, ok := db.subgraphLookup[id]; ok {
			subgraph.classes = append(subgraph.classes, classNames...)
		}
	}
}

// addSubGraph takes the title's text only: the parser never gives it a type, so its label
// type is always upstream's default.
func (db *erDb) addSubGraph(idText string, list []docItem, titleText string) string {
	id := jsstr.Trim(idText)
	seen := map[string]bool{}
	var nodes []string
	dir := ""
	for _, item := range list {
		if item.isDir {
			dir = item.value
			continue
		}
		trimmed := jsstr.Trim(item.value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		// A node belongs to the first subgraph that lists it.
		if !db.claimed[item.value] {
			nodes = append(nodes, item.value)
		}
	}
	for _, node := range nodes {
		db.claimed[node] = true
	}
	subgraph := &erSubgraph{
		id:        id,
		nodes:     nodes,
		title:     jsstr.Trim(titleText),
		dir:       dir,
		labelType: labelType(""),
	}
	db.subgraphs = append(db.subgraphs, subgraph)
	db.subgraphLookup[id] = subgraph
	return id
}

func (db *erDb) setAccTitle(text string) {
	db.accTitle = jsstr.TrimStart(text)
}

func (db *erDb) setAccDescription(text string) {
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
