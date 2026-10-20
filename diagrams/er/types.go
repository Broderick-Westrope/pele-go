package er

type cardinality string

type identification string

type erAttribute struct {
	typ     string
	name    string
	keys    []string
	comment string
}

type erEntity struct {
	id         string
	label      string
	alias      string
	attributes []erAttribute
	cssClasses string
	cssStyles  []string
}

// relSpec: cardB sits at entityA's end of the line and cardA at entityB's, as in Mermaid.
type relSpec struct {
	cardA   cardinality
	relType identification
	cardB   cardinality
}

// erRelationship: entityA and entityB hold an entity's id, or a subgraph's id when the name
// was a subgraph at the time.
type erRelationship struct {
	entityA string
	roleA   string
	entityB string
	relSpec relSpec
}

type erClassDef struct {
	id         string
	styles     []string
	textStyles []string
}

type erSubgraph struct {
	id        string
	nodes     []string
	title     string
	classes   []string
	cssStyles []string
	// dir is "" where upstream's is undefined.
	dir       string
	labelType string
}

// docItem is a string, or a direction statement when isDir is set.
type docItem struct {
	value string
	isDir bool
}
