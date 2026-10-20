package railroad

type nodeType int

const (
	nTerminal nodeType = iota
	nNonTerminal
	nSpecial
	nSequence
	nChoice
	nOptional
	nRepetition
)

// node is upstream's RailroadNode.
type node struct {
	typ nodeType
	// value is a terminal's value, a non-terminal's name or a special's text.
	value string
	// kids are a sequence's elements, a choice's alternatives, or the one element of an
	// optional or a repetition.
	kids []*node
	// max is +Inf when the count has no upper bound.
	min, max float64
}

type rule struct {
	name       string
	definition *node
}
