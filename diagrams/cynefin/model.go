package cynefin

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

type cynefinTransition struct {
	from, to string
	// label is "" for none.
	label string
}

type cynefinModel struct {
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
	// domains holds the items of each domain the source declares. All five are drawn
	// either way.
	domains     map[string][]string
	transitions []cynefinTransition
}

type cynefinDb struct {
	domains     map[string][]string
	transitions []cynefinTransition
}

// setDomains sets each block's items; a later block for a domain replaces an earlier one.
func (db *cynefinDb) setDomains(blocks []domainBlockAst) {
	for _, block := range blocks {
		db.domains[block.domain] = block.items
	}
}

func (db *cynefinDb) setTransitions(transitions []transitionAst) {
	db.transitions = nil
	for _, t := range transitions {
		if t.from == t.to {
			continue
		}
		label := ""
		if t.label != nil {
			label = *t.label
		}
		db.transitions = append(db.transitions, cynefinTransition{t.from, t.to, label})
	}
}

func buildCynefin(ast cynefinAst, title *string) cynefinModel {
	db := &cynefinDb{domains: map[string][]string{}}
	db.setDomains(ast.domains)
	db.setTransitions(ast.transitions)
	m := cynefinModel{domains: db.domains, transitions: db.transitions}
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
