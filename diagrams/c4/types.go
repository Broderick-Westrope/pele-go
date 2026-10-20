package c4

// A `$key="value"` argument.
type named struct {
	key, value string
}

// attr is a positional string, or a named argument when n is set.
type attr struct {
	s string
	n *named
}

// Optional fields are "" where upstream's are undefined: nothing that reads them tells the
// two apart.
type element struct {
	alias string
	label string
	// A named argument written where the alias or the label belongs. It is applied to the
	// field it names.
	aliasAttr    *named
	labelAttr    *named
	parent       *element
	typ          string
	descr        string
	techn        string
	sprite       string
	tags         string
	link         string
	bgColor      string
	fontColor    string
	borderColor  string
	shadowing    string
	shape        string
	legendText   string
	legendSprite string
	// kind is Mermaid's typeC4Shape on shapes: person, external_system_db, container_queue
	// and so on.
	kind string
	// nodeType is set on deployment nodes: node, nodeL or nodeR.
	nodeType string
}

type rel struct {
	typ       string
	from      string
	to        string
	label     string
	fromAttr  *named
	toAttr    *named
	labelAttr *named
	techn     string
	descr     string
	sprite    string
	tags      string
	link      string
	textColor string
	lineColor string
	// NaN where parseInt found no number; 0 where unset.
	offsetX float64
	offsetY float64
}
