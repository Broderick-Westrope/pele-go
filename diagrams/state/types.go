package state

type stateNote struct {
	position string
	text     string
}

// descr is a description that is a string or a list of them. The zero value is upstream's
// undefined, which every use treats as the empty string.
type descr struct {
	isList bool
	s      string
	list   []string
}

func (d descr) truthy() bool { return d.isList || d.s != "" }

// stmt is upstream's Stmt: one of the types below.
type stmt interface{ isStmt() }

// textStmt is the text of a statement that has no effect in a document.
type textStmt string

type stateStmt struct {
	id string
	// "" is undefined.
	typ          string
	description  descr
	descriptions []string
	// nil is undefined.
	doc        *[]stmt
	note       *stateNote
	start      *bool
	classes    []string
	styles     []string
	textStyles []string
}

type relationStmt struct {
	state1, state2 *stateStmt
	description    string
}

type classDefStmt struct{ id, classes string }

type styleStmt struct{ id, styleClass string }

type applyClassStmt struct{ id, styleClass string }

type directionStmt struct{ value string }

type clickStmt struct {
	id           *stateStmt
	url, tooltip string
}

func (textStmt) isStmt()        {}
func (*stateStmt) isStmt()      {}
func (*relationStmt) isStmt()   {}
func (*classDefStmt) isStmt()   {}
func (*styleStmt) isStmt()      {}
func (*applyClassStmt) isStmt() {}
func (*directionStmt) isStmt()  {}
func (*clickStmt) isStmt()      {}

type stateRelation struct {
	id1, id2      string
	relationTitle string
}

type stateClassDef struct {
	id         string
	styles     []string
	textStyles []string
}

type stateLink struct {
	url, tooltip string
}
