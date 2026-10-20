package block

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

const maxSpaces = 100000

func typeStr2Type(typeStr string) string {
	switch typeStr {
	case "[]":
		return "square"
	case "()":
		return "round"
	case "(())":
		return "circle"
	case ">]":
		return "rect_left_inv_arrow"
	case "{}":
		return "diamond"
	case "{{}}":
		return "hexagon"
	case "([])":
		return "stadium"
	case "[[]]":
		return "subroutine"
	case "[()]":
		return "cylinder"
	case "((()))":
		return "doublecircle"
	case "[//]":
		return "lean_right"
	case `[\\]`:
		return "lean_left"
	case `[/\]`:
		return "trapezoid"
	case `[\/]`:
		return "inv_trapezoid"
	case "<[]>":
		return "block_arrow"
	default:
		return "na"
	}
}

func edgeStrToEdgeData(typeStr string) string {
	t := jsstr.Trim(typeStr)
	if t == "" {
		return ""
	}
	switch t[len(t)-1] {
	case 'x':
		return "arrow_cross"
	case 'o':
		return "arrow_circle"
	case '>':
		return "arrow_point"
	default:
		return ""
	}
}

func edgeStrToEdgeStartData(typeStr string) string {
	t := jsstr.Trim(typeStr)
	if t == "" {
		return "arrow_open"
	}
	switch t[0] {
	case 'x':
		return "arrow_cross"
	case 'o':
		return "arrow_circle"
	case '<':
		return "arrow_point"
	default:
		return "arrow_open"
	}
}

func edgeStrToThickness(typeStr string) string {
	if strings.Contains(typeStr, "==") {
		return "thick"
	}
	return "normal"
}

func edgeStrToPattern(typeStr string) string {
	if strings.Contains(typeStr, ".-") {
		return "dotted"
	}
	return "solid"
}

type frame struct {
	list     []*Block
	at       int
	parent   *Block
	children []*Block
	columns  float64
}

func newFrame(list []*Block, parent *Block) *frame {
	columns := -1.0
	for _, block := range list {
		if block.typ == "column-setting" {
			columns = block.columns
			break
		}
	}
	return &frame{list: list, parent: parent, columns: columns}
}

// blockDb is upstream's BlockDb, without its warn option: index.ts gives none.
type blockDb struct {
	title  *string
	root   *Block
	blocks []*Block
	edges  []*Block
	// classes is only read by id.
	classes map[string]*blockClassDef
	// Mermaid keeps the root in here too, which makes a block called `root` vanish. Here it stays out.
	byId omap.Map[string, *Block]

	// Ids that a class or style statement named before any block had them.
	pending   omap.Set[string]
	edgeCount map[string]int
	ids       int
	colors    int
	spaces    float64
}

func newBlockDb() *blockDb {
	return &blockDb{
		root:      &Block{id: "root", hasID: true, typ: "composite", columns: -1, widthInColumns: 1, colorIndex: -1},
		classes:   map[string]*blockClassDef{},
		edgeCount: map[string]int{},
	}
}

// Mermaid's ids are random. These are counted, so the same text always gives the same output.
func (db *blockDb) generateId() string {
	db.ids++
	return "id-" + jsnum.Format(float64(db.ids))
}

func (db *blockDb) addStyleClass(id, styleAttributes string) {
	found, ok := db.classes[id]
	if !ok {
		found = &blockClassDef{id: id}
		db.classes[id] = found
	}
	for _, attribute := range strings.Split(styleAttributes, ",") {
		fixed := attribute
		if semi := strings.IndexByte(attribute, ';'); semi != -1 {
			fixed = attribute[:semi] + attribute[semi+1:]
		}
		fixed = jsstr.Trim(fixed)
		if strings.Contains(attribute, "color") {
			found.textStyles = append(found.textStyles, strings.Replace(strings.Replace(fixed, "fill", "bgFill", 1), "color", "fill", 1))
		}
		found.styles = append(found.styles, fixed)
	}
}

func (db *blockDb) addStyle2Node(id, styles string) {
	db.named(id).styles = strings.Split(styles, ",")
}

// Mermaid looks each id up before trimming it, so `class a, b hot` misses b. Here it is trimmed first.
func (db *blockDb) setCssClass(itemIds, cssClassName string) {
	for _, id := range strings.Split(itemIds, ",") {
		b := db.named(jsstr.Trim(id))
		b.classes = append(b.classes, cssClassName)
	}
}

// Mermaid throws on a style for an unknown id, and a class for one makes the block of that id
// disappear when it is declared later. Here both wait for the block.
func (db *blockDb) named(id string) *Block {
	b, ok := db.byId.Get(id)
	if !ok {
		b = &Block{id: id, hasID: true, typ: "na", columns: -1, widthInColumns: 1, colorIndex: -1}
		db.byId.Set(id, b)
		db.pending.Add(id)
	}
	return b
}

func (db *blockDb) setHierarchy(statements []*Block) {
	stack := []*frame{newFrame(statements, db.root)}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		if f.at == len(f.list) {
			f.parent.children = f.children
			stack = stack[:len(stack)-1]
			continue
		}
		block := f.list[f.at]
		f.at++
		switch block.typ {
		case "classDef":
			db.addStyleClass(block.id, block.css)
			continue
		case "applyClass":
			db.setCssClass(block.id, block.styleClass)
			continue
		case "applyStyles":
			if block.stylesStr != "" {
				db.addStyle2Node(block.id, block.stylesStr)
			}
			continue
		case "column-setting":
			f.parent.columns = block.columns
			continue
		case "edge":
			count := db.edgeCount[block.id] + 1
			db.edgeCount[block.id] = count
			block.id = jsnum.Format(float64(count)) + "-" + block.id
			db.edges = append(db.edges, block)
			continue
		}

		// A composite whose header was a link statement has no id of its own.
		if !block.hasID {
			block.id, block.hasID = db.generateId(), true
		}
		if block.label == "" {
			if block.typ != "composite" {
				block.label = block.id
			}
		}
		known, isKnown := db.byId.Get(block.id)
		fresh := !isKnown || db.pending.Delete(block.id)
		if !fresh {
			if block.typ != "na" {
				known.typ = block.typ
			}
			if block.label != block.id {
				known.label = block.label
			}
		} else {
			if isKnown {
				block.classes = known.classes
				block.styles = known.styles
			}
			if block.typ == "composite" {
				block.colorIndex = db.colors
				db.colors++
			}
			db.byId.Set(block.id, block)
		}

		inner := block.children
		block.children = nil
		if block.typ == "space" {
			width := block.width
			if !(width <= maxSpaces-db.spaces) {
				e := perr.New("A block diagram can hold at most "+jsnum.Format(maxSpaces)+" space blocks.", perr.Limit)
				e.Type = "block"
				perr.Throw(e)
			}
			db.spaces += max(width, 0)
			for j := 0; float64(j) < width; j++ {
				dup := *block
				dup.id = block.id + "-" + jsnum.Format(float64(j))
				dup.children = nil
				db.byId.Set(dup.id, &dup)
				f.children = append(f.children, &dup)
			}
		} else if fresh {
			f.children = append(f.children, block)
		}
		if len(inner) > 0 {
			stack = append(stack, newFrame(inner, block))
		}
	}
	db.blocks = db.root.children
}
