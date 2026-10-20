package kanban

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
	"github.com/Broderick-Westrope/pele-go/internal/yaml"
)

// kanbanNode is upstream's KanbanNode, and with items its KanbanSection. Empty strings stand
// for undefined: every use treats the two alike.
type kanbanNode struct {
	id                                           string
	level                                        int
	label                                        string
	icon, assigned, ticket, priority, cssClasses string
	items                                        []*kanbanNode
}

// kanbanDb is upstream's KanbanDb and KanbanModel.
type kanbanDb struct {
	title    string
	sections []*kanbanNode

	last         *kanbanNode
	sectionLevel int
	misplaced    *kanbanNode
}

func kanbanError(message string, code perr.Code) {
	e := perr.New(message, code)
	e.Type = "kanban"
	perr.Throw(e)
}

func (db *kanbanDb) GetType(start, end string) int {
	return common.GetType(start, end)
}

func (db *kanbanDb) AddNode(level int, id, descr string, _ int, shapeData *string) {
	node := &kanbanNode{id: id, level: level, label: descr}

	if shapeData != nil {
		src := "{\n" + *shapeData + "\n}"
		if strings.Contains(*shapeData, "\n") {
			src = *shapeData + "\n"
		}
		doc, err := yaml.Parse(src)
		if err != nil {
			kanbanError(`Metadata of "`+node.id+`" is not valid YAML. `+err.Error(), perr.Syntax)
		}
		// Mermaid throws a TypeError on metadata that is empty or not a mapping. Here it is ignored.
		if doc, ok := doc.(*jsobj.Object); ok {
			field := func(key string) string {
				v, _ := doc.Get(key)
				if !jsobj.Truthy(v) {
					return ""
				}
				return jsobj.ToString(v)
			}
			shape := field("shape")
			if jsstr.ChangesWhenLowered(shape) || strings.Contains(shape, "_") {
				kanbanError("No such shape: "+shape+". Shape names should be lowercase.", perr.Semantic)
			}
			if v, _ := doc.Get("label"); jsobj.Truthy(v) {
				node.label = jsobj.ToString(v)
			}
			node.icon = field("icon")
			node.assigned = field("assigned")
			node.ticket = field("ticket")
			node.priority = field("priority")
		}
	}

	// A node indented less than the first column becomes a card, and Mermaid rejects whatever follows it.
	if db.misplaced != nil {
		kanbanError(`Items without section detected, found section ("`+db.misplaced.label+`")`, perr.Semantic)
	}
	if len(db.sections) == 0 {
		db.sectionLevel = level
	}
	db.last = node
	if level == db.sectionLevel {
		node.items = []*kanbanNode{}
		db.sections = append(db.sections, node)
	} else {
		section := db.sections[len(db.sections)-1]
		section.items = append(section.items, node)
		if level < db.sectionLevel {
			db.misplaced = node
		}
	}
}

// DecorateNode sets the last node's icon or classes. Mermaid throws a TypeError when a
// decoration comes before any node. Here it is ignored.
func (db *kanbanDb) DecorateNode(d common.Decoration) {
	node := db.last
	if node == nil {
		return
	}
	if d.Icon != nil && *d.Icon != "" {
		node.icon = *d.Icon
	}
	if d.Class != nil && *d.Class != "" {
		node.cssClasses = *d.Class
	}
}
