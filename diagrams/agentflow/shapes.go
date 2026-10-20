package agentflow

import "github.com/Broderick-Westrope/pele-go/internal/diagrams/flowchart"

// shapeAliases are the author-facing names for the shapes that carry meaning in an agentflow.
var shapeAliases = map[string]string{
	"task":     "roundedRect",
	"tool":     "subroutine",
	"input":    "lean-right",
	"decision": "diamond",
	"refdoc":   "lin-doc",
	"action":   "hexagon",
	"round":    "rect",
}

var removedShapes = setOf(
	"doc", "stadium", "terminal", "circle", "trapezoid", "inv_trapezoid", "inv-trapezoid", "doublecircle",
	"double-circle", "typeDeclaration", "procs", "lean_left", "lean-left", "in-out", "cylinder", "ellipse", "odd",
	"tag-rect", "tagged-rectangle", "delay", "half-rounded-rectangle", "lin-rect", "lined-rectangle", "win-pane",
	"window-pane", "curv-trap", "curved-trapezoid",
)

var allowedShapes = setOf(
	"roundedRect", "subroutine", "subprocess", "subproc", "framed-rectangle", "lean-right", "diamond", "lin-doc",
	"lined-document", "hexagon", "hex", "connector", "collapsedGroup",
)

const defaultShape = "roundedRect"

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// resolveShapeAlias is the shape an author-facing name stands for. "" is undefined.
func resolveShapeAlias(shape string) string {
	if s, ok := shapeAliases[shape]; ok {
		return s
	}
	return shape
}

// isValidShape is flowchart's check. Mermaid looks a shape name up with `in` on a plain
// object, so `constructor` passes there too.
func isValidShape(name string) bool {
	return flowchart.IsValidShape(name) || name == "constructor"
}

// normaliseNodeShapes reduces every node's shape to one agentflow draws, reporting the
// removed and unsupported ones.
func normaliseNodeShapes(nodes []*graphNode, db *agentflowDb) {
	for _, node := range nodes {
		if node.isGroup {
			continue
		}
		if node.shape == "" || node.shape == "squareRect" || node.shape == "rect" {
			node.shape = defaultShape
		}
		if removedShapes[node.shape] {
			db.emitError("SHAPE_REMOVED", `shape "`+node.shape+`" was removed in v0.8.1, using "`+defaultShape+`"`, node.id)
			node.shape = defaultShape
		} else if !allowedShapes[node.shape] {
			db.emitWarning("SHAPE_UNSUPPORTED", `shape "`+node.shape+`" is not supported, using "`+defaultShape+`"`, node.id)
			node.shape = defaultShape
		}
	}
}
