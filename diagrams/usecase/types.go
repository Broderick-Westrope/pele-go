package usecase

import (
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
)

// span is a start and end offset in the source, end exclusive, in UTF-16 code units.
type span [2]int

type actor struct {
	id        string
	label     string
	labelType string
	typ       string
	icon      string
	business  bool
	// stereotype and parentID are "" where upstream's are undefined: neither is ever empty.
	stereotype string
	parentID   string
	classes    []string
	styles     []string
}

type useCase struct {
	id         string
	label      string
	labelType  string
	shape      string
	business   bool
	stereotype string
	parentID   string
	classes    []string
	styles     []string
}

type systemBoundary struct {
	id        string
	label     string
	labelType string
	typ       string
	members   []string
	classes   []string
	styles    []string
}

// The arrow types, in the order of upstream's Arrow enum.
const (
	arrowSolidArrow = iota
	arrowBackArrow
	arrowLineSolid
	arrowCircleArrow
	arrowCrossArrow
	arrowCircleArrowReversed
	arrowCrossArrowReversed
)

type relationship struct {
	id         string
	explicitID bool
	source     string
	target     string
	typ        string
	arrowType  int
	// label and labelType are "" where upstream's are undefined; an empty label draws as none.
	label     string
	labelType string
	minlen    int
	classes   []string
	styles    []string
	animate   bool
	animation string
}

type usecaseNote struct {
	id        string
	target    string
	label     string
	labelType string
}

type usecaseJSONNode struct {
	id    string
	value *jsobj.Object
	// The keys of each object in the order they were written, by JSON Pointer.
	propertyOrder map[string][]string
	classes       []string
	styles        []string
}

type classDef struct {
	id     string
	styles []string
}

// usecaseModel is upstream's UsecaseModel without its record of each statement's spans
// (statements, headerSpan, source), which is there for editors and which nothing drawn reads.
type usecaseModel struct {
	actors           omap.Map[string, *actor]
	useCases         omap.Map[string, *useCase]
	systemBoundaries omap.Map[string, *systemBoundary]
	relationships    []*relationship
	notes            omap.Map[string, *usecaseNote]
	jsonNodes        omap.Map[string, *usecaseJSONNode]
	classDefs        omap.Map[string, *classDef]
	direction        string
	title            string
	accTitle         string
	accDescr         string
}

func emptyModel() *usecaseModel {
	return &usecaseModel{direction: "LR"}
}
