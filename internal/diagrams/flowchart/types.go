// Package flowchart is upstream's src/diagrams/flowchart: the flowchart parser, model, graph
// and renderer. It is internal because swimlane and agentflow reuse it, as upstream's do;
// the public diagram type is diagrams/flowchart.
package flowchart

import "github.com/Broderick-Westrope/pele-go/internal/jsobj"

// LabelType is how a label is written: "text", "string" or "markdown".
type LabelType string

// FlowText is a label as written.
type FlowText struct {
	Text string
	Type LabelType
}

// FlowNode is a vertex. Empty strings and zero numbers stand for upstream's undefined: every
// use treats the two alike.
type FlowNode struct {
	ID         string
	Text       string
	LabelType  LabelType
	Type       string
	Styles     []string
	Classes    []string
	Dir        string
	Link       string
	LinkTarget string
	Icon       string
	Form       string
	Pos        string
	Img        string
	Constraint string
	// AssetWidth and AssetHeight are 0 when unset; NaN and 0 are drawn as unset too.
	AssetWidth  float64
	AssetHeight float64
}

// FlowEdge is an edge as parsed.
type FlowEdge struct {
	Start     string
	End       string
	Type      string
	Text      string
	LabelType LabelType
	Stroke    string
	// Length is 0 for upstream's undefined: a parsed length is at least 1.
	Length          float64
	Style           []string
	Classes         []string
	ID              string
	IsUserDefinedID bool
	// Interpolate is any YAML value metadata gave as the curve, a string from linkStyle, or
	// nil. Only a string names a curve, but any truthy value hides the defaults.
	Interpolate any
	Animate     any
	Animation   any
}

// FlowClassDef is a classDef.
type FlowClassDef struct {
	ID         string
	Styles     []string
	TextStyles []string
}

// FlowSubgraph is a subgraph. Dir "" is undefined.
type FlowSubgraph struct {
	ID        string
	Nodes     []string
	Title     string
	Classes   []string
	Dir       string
	LabelType LabelType
	Metadata  *jsobj.Object
}

// LinkInfo is what a link token says about an edge. Text is nil and ID "" when unset; a
// LINK_ID token is never empty.
type LinkInfo struct {
	Type   string
	Stroke string
	Length float64
	Text   *FlowText
	ID     string
}

// docItem is upstream's DocItem: node ids (a string is one id), or a direction statement.
type docItem struct {
	ids []string
	dir string
}
