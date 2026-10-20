package block

// Block is one shape for blocks, edges, and the statements that set columns, classes, and
// styles, as in Mermaid. Where upstream's field can be undefined, the zero value or the
// default upstream reads it with (`?? -1`, `?? 1`) stands for it.
type Block struct {
	id string
	// hasID is false only for a composite whose header was a link statement.
	hasID bool
	typ   string
	label string
	// children is nil where upstream's is undefined or empty: every use treats the two alike.
	children []*Block
	// columns is the grid width, -1 when unset.
	columns float64
	// widthInColumns is 1 when unset.
	widthInColumns float64
	// width is the number of cells a `space` statement asked for.
	width      float64
	directions []string
	classes    []string
	styles     []string
	// colorIndex is -1 when unset.
	colorIndex     int
	start          string
	end            string
	arrowTypeStart string
	arrowTypeEnd   string
	thickness      string
	pattern        string
	css            string
	styleClass     string
	stylesStr      string
}

type blockClassDef struct {
	id         string
	styles     []string
	textStyles []string
}
