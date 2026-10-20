package requirement

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

const (
	requirementTypeRequirement            = "Requirement"
	requirementTypeFunctionalRequirement  = "Functional Requirement"
	requirementTypeInterfaceRequirement   = "Interface Requirement"
	requirementTypePerformanceRequirement = "Performance Requirement"
	requirementTypePhysicalRequirement    = "Physical Requirement"
	requirementTypeDesignConstraint       = "Design Constraint"

	riskLow    = "Low"
	riskMedium = "Medium"
	riskHigh   = "High"

	verifyAnalysis      = "Analysis"
	verifyDemonstration = "Demonstration"
	verifyInspection    = "Inspection"
	verifyTest          = "Test"

	relContains  = "contains"
	relCopies    = "copies"
	relDerives   = "derives"
	relSatisfies = "satisfies"
	relVerifies  = "verifies"
	relRefines   = "refines"
	relTraces    = "traces"
)

// maxStyleCopies: applying a class copies its declarations onto each node, so a short text can
// ask for hundreds of millions of copies. Past this many the diagram is refused.
const maxStyleCopies = 1_000_000

// requirementDb is upstream's RequirementDb, which is also the model the renderer reads.
type requirementDb struct {
	direction    string
	requirements omap.Map[string, *requirement]
	elements     omap.Map[string, *requirementElement]
	relations    []relation
	classes      omap.Map[string, *requirementClass]
	// title, accTitle and accDescr are "" where upstream's are undefined: every use treats
	// the two alike.
	title    string
	accTitle string
	accDescr string

	// The grammar reports a body's fields before the requirement or element they belong to.
	newID           string
	newText         string
	newRisk         string
	newVerifyMethod string
	newElementType  string
	newDocRef       string
	styleCopies     int
}

func newRequirementDb() *requirementDb {
	return &requirementDb{direction: "TB"}
}

func (db *requirementDb) copy(target *[]string, styles []string) {
	db.styleCopies += len(styles)
	if db.styleCopies > maxStyleCopies {
		e := perr.New("Requirement diagram applies too many style declarations.", perr.Limit)
		e.Type = "requirement"
		perr.Throw(e)
	}
	*target = append(*target, styles...)
}

func (db *requirementDb) setDirection(dir string) {
	db.direction = dir
}

func (db *requirementDb) addRequirement(name, typ string) *requirement {
	req, ok := db.requirements.Get(name)
	if !ok {
		req = &requirement{
			name:          name,
			typ:           typ,
			requirementID: db.newID,
			text:          db.newText,
			risk:          db.newRisk,
			verifyMethod:  db.newVerifyMethod,
			styling:       styling{cssStyles: []string{}, classes: []string{"default"}},
		}
		db.requirements.Set(name, req)
	}
	db.newID, db.newText, db.newRisk, db.newVerifyMethod = "", "", "", ""
	return req
}

func (db *requirementDb) setNewReqID(id string) {
	db.newID = id
}

func (db *requirementDb) setNewReqText(text string) {
	db.newText = text
}

func (db *requirementDb) setNewReqRisk(risk string) {
	db.newRisk = risk
}

func (db *requirementDb) setNewReqVerifyMethod(verifyMethod string) {
	db.newVerifyMethod = verifyMethod
}

func (db *requirementDb) addElement(name string) *requirementElement {
	el, ok := db.elements.Get(name)
	if !ok {
		el = &requirementElement{
			name:    name,
			typ:     db.newElementType,
			docRef:  db.newDocRef,
			styling: styling{cssStyles: []string{}, classes: []string{"default"}},
		}
		db.elements.Set(name, el)
	}
	db.newElementType, db.newDocRef = "", ""
	return el
}

func (db *requirementDb) setNewElementType(typ string) {
	db.newElementType = typ
}

func (db *requirementDb) setNewElementDocRef(docRef string) {
	db.newDocRef = docRef
}

func (db *requirementDb) addRelationship(typ, src, dst string) {
	db.relations = append(db.relations, relation{typ: typ, src: src, dst: dst})
}

func (db *requirementDb) node(id string) *styling {
	if req, ok := db.requirements.Get(id); ok {
		return &req.styling
	}
	if el, ok := db.elements.Get(id); ok {
		return &el.styling
	}
	return nil
}

func (db *requirementDb) setCssStyle(ids, styles []string) {
	for _, id := range ids {
		node := db.node(id)
		// Mermaid stops at the first unknown id, leaving the rest of the list unstyled.
		if node == nil {
			return
		}
		for _, s := range styles {
			db.copy(&node.cssStyles, strings.Split(s, ","))
		}
	}
}

func (db *requirementDb) setClass(ids, classNames []string) {
	for _, id := range ids {
		node := db.node(id)
		if node == nil {
			continue
		}
		for _, name := range classNames {
			node.classes = append(node.classes, name)
			if class, ok := db.classes.Get(name); ok {
				db.copy(&node.cssStyles, class.styles)
			}
		}
	}
}

func (db *requirementDb) defineClass(ids, style []string) {
	for _, id := range ids {
		class, ok := db.classes.Get(id)
		if !ok {
			class = &requirementClass{id: id, styles: []string{}, textStyles: []string{}}
			db.classes.Set(id, class)
		}
		for _, s := range style {
			if strings.Contains(s, "color") {
				class.textStyles = append(class.textStyles, strings.Replace(s, "fill", "bgFill", 1))
			}
			class.styles = append(class.styles, s)
		}
		var split []string
		for _, s := range style {
			split = append(split, strings.Split(s, ",")...)
		}
		for _, req := range db.requirements.All() {
			if hasClass(req.classes, id) {
				db.copy(&req.cssStyles, split)
			}
		}
		for _, el := range db.elements.All() {
			if hasClass(el.classes, id) {
				db.copy(&el.cssStyles, split)
			}
		}
	}
}

func hasClass(classes []string, id string) bool {
	for _, c := range classes {
		if c == id {
			return true
		}
	}
	return false
}

func (db *requirementDb) setAccTitle(text string) {
	db.accTitle = jsstr.TrimStart(text)
}

func (db *requirementDb) setAccDescription(text string) {
	db.accDescr = common.CollapseIndentedBreaks(text)
}
