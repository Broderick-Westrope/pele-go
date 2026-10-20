package treemap

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

type treemapNode struct {
	name string
	// leaf nodes have no children; sections have a list, which may be empty.
	leaf     bool
	children []*treemapNode
	// value is set on leaves only.
	value float64
	// classSelector is nil where upstream's is undefined.
	classSelector     *string
	cssCompiledStyles []string
}

type treemapItem struct {
	level             int
	name              string
	leaf              bool
	value             float64
	classSelector     *string
	cssCompiledStyles []string
}

type treemapModel struct {
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
	roots                     []*treemapNode
	classes                   map[string][]string
}

// buildHierarchy places each row under the nearest section above it that is indented less.
func buildHierarchy(items []treemapItem) []*treemapNode {
	var roots []*treemapNode
	type frame struct {
		node  *treemapNode
		level int
	}
	var stack []frame
	for _, item := range items {
		node := &treemapNode{name: item.name, leaf: item.leaf, classSelector: item.classSelector, cssCompiledStyles: item.cssCompiledStyles}
		if item.leaf {
			node.value = item.value
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= item.level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, node)
		} else {
			parent := stack[len(stack)-1].node
			parent.children = append(parent.children, node)
		}
		if !item.leaf {
			stack = append(stack, frame{node, item.level})
		}
	}
	return roots
}

func buildTreemap(ast treemapAst, title *string) *treemapModel {
	classes := map[string][]string{}
	for _, row := range ast.rows {
		if !row.classDef {
			continue
		}
		styles := classes[row.className]
		text := ""
		if row.styleText != nil {
			text = *row.styleText
		}
		// An escaped comma stays in the value; the others separate declarations.
		text = strings.ReplaceAll(text, `\,`, "§§§")
		text = strings.ReplaceAll(text, ",", ";")
		text = strings.ReplaceAll(text, "§§§", ",")
		styles = append(styles, strings.Split(text, ";")...)
		classes[row.className] = styles
	}

	var items []treemapItem
	for _, row := range ast.rows {
		if row.classDef {
			continue
		}
		item := row.item
		var styles []string
		if item.classSelector != nil && *item.classSelector != "" {
			styles = classes[*item.classSelector]
		}
		level := 0
		if row.indent != nil {
			level = *row.indent
		}
		items = append(items, treemapItem{
			level:             level,
			name:              item.name,
			leaf:              item.leaf,
			value:             item.value,
			classSelector:     item.classSelector,
			cssCompiledStyles: styles,
		})
	}

	m := &treemapModel{roots: buildHierarchy(items), classes: classes}
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
