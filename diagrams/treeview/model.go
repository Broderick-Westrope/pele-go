package treeview

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type nodeType int

const (
	nodeFile nodeType = iota
	nodeDirectory
)

type treeNode struct {
	id       int
	level    int
	name     string
	nodeType nodeType
	// icon is an icon name as written (`pack:name`, `file`, `folder`), or `none` to hide the
	// icon. "" is upstream's undefined, as are cssClass and description.
	icon        string
	cssClass    string
	description string
	children    []*treeNode
}

type treeViewModel struct {
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
	// root is a directory named `/` at level -1 that holds the top-level entries.
	root  *treeNode
	count int
}

// treeViewDb is Mermaid's treeView database: nodes arrive in source order with their
// indentation.
type treeViewDb struct {
	count int
	root  *treeNode
	stack []*treeNode
}

func newTreeViewDb() *treeViewDb {
	root := &treeNode{level: -1, name: "/", nodeType: nodeDirectory}
	return &treeViewDb{count: 1, root: root, stack: []*treeNode{root}}
}

func (db *treeViewDb) addNode(level int, name string, typ nodeType, cssClass, icon, description string) {
	for level <= db.stack[len(db.stack)-1].level {
		db.stack = db.stack[:len(db.stack)-1]
	}
	node := &treeNode{id: db.count, level: level, name: name, nodeType: typ, icon: icon, cssClass: cssClass, description: description}
	db.count++
	parent := db.stack[len(db.stack)-1]
	parent.children = append(parent.children, node)
	db.stack = append(db.stack, node)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func populate(ast treeViewAst, db *treeViewDb) {
	for _, node := range ast.nodes {
		level := 0
		if node.indent != nil {
			level = *node.indent
		}
		name := node.name
		isDirectory := strings.HasSuffix(name, "/")
		if isDirectory {
			name = name[:len(name)-1]
		}
		icon := ""
		if node.iconAnnotation != nil {
			icon = *node.iconAnnotation
			if icon == "" {
				icon = "none"
			}
		}
		typ := nodeFile
		if isDirectory {
			typ = nodeDirectory
		}
		db.addNode(level, name, typ, deref(node.classAnnotation), icon, deref(node.descAnnotation))
	}
}

// readTreeView reads the source to a syntax tree, accepting box-drawing input. Errors name
// lines of the original text.
func readTreeView(source string) treeViewAst {
	text, lineMap := preprocessBoxDrawing(source)
	if len(lineMap) == 0 {
		return parseTreeView(text)
	}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		e, ok := r.(*perr.Error)
		if !ok {
			panic(r)
		}
		message := e.Message
		if head := strings.IndexByte(message, '\n'); head == -1 {
			message = remapErrorLines(message, lineMap)
		} else {
			message = remapErrorLines(message[:head], lineMap) + message[head:]
		}
		line, ok := lineMap[e.Line]
		if !ok {
			line = e.Line
		}
		perr.Throw(&perr.Error{Code: e.Code, Message: message, Type: e.Type, Line: line, Column: e.Column, Snippet: e.Snippet})
	}()
	return parseTreeView(text)
}

func buildTreeView(ast treeViewAst, title *string) treeViewModel {
	db := newTreeViewDb()
	populate(ast, db)
	m := treeViewModel{root: db.root, count: db.count}
	if ast.title != nil && *ast.title != "" {
		m.title = *ast.title
	} else if title != nil {
		m.title = *title
	}
	if ast.accTitle != nil && *ast.accTitle != "" {
		m.accTitle = jsstr.TrimStart(*ast.accTitle)
	}
	if ast.accDescr != nil && *ast.accDescr != "" {
		m.accDescr = common.CollapseIndentedBreaks(*ast.accDescr)
	}
	return m
}
