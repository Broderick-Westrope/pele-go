package agentflow

import "github.com/Broderick-Westrope/pele-go/internal/jsobj"

// labelKind is upstream's LabelType, how a label is written: "text", "string" or
// "markdown".
type labelKind string

// flowText is a label as written.
type flowText struct {
	text string
	typ  labelKind
}

// loc is a source span as Jison reports it: lines start at 1, columns at 0.
type loc struct {
	firstLine, firstColumn, lastLine, lastColumn int
}

// agentNode is a vertex or a connector. typ "" is upstream's undefined; link and linkTarget
// are nil when undefined, as an empty link is kept.
type agentNode struct {
	id          string
	text        string
	labelType   labelKind
	typ         string
	styles      []string
	classes     []string
	props       *jsobj.Object
	link        *string
	linkTarget  *string
	metadata    *jsobj.Object
	isConnector bool
}

// agentEdge is an edge as parsed. length 0 and the empty strings are undefined; interpolate
// is any value metadata gave as the curve, a string, or unset.
type agentEdge struct {
	start           string
	end             string
	typ             string
	text            string
	labelType       labelKind
	stroke          string
	edgeSemantic    string
	length          float64
	style           []string
	classes         []string
	id              string
	isUserDefinedID bool
	interpolate     any
	hasInterpolate  bool
	animate         any
	animation       any
	metadata        *jsobj.Object
}

type agentClassDef struct {
	id         string
	styles     []string
	textStyles []string
}

// agentSubgraph is a flow. dir "" is undefined.
type agentSubgraph struct {
	id        string
	nodes     []string
	title     string
	classes   []string
	dir       string
	labelType labelKind
	metadata  *jsobj.Object
}

// linkInfo is what a link token says about an edge. text is nil and id "" when unset.
type linkInfo struct {
	typ          string
	stroke       string
	length       float64
	edgeSemantic string
	text         *flowText
	id           string
}

// docItem is upstream's DocItem: node ids (a string is one id), or a direction statement.
type docItem struct {
	ids []string
	dir string
}

type elementPosition struct {
	startLine, startColumn, endLine, endColumn, startIndex, endIndex int
}

type elementMapping struct {
	id       string
	typ      string
	position *elementPosition
}

// diagnostic is a finding about an agentflow's meaning. nodeID and edgeID are "" when unset,
// and position nil.
type diagnostic struct {
	id       string
	severity string
	message  string
	nodeID   string
	edgeID   string
	position *elementPosition
}
