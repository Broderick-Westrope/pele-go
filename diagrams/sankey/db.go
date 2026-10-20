package sankey

type sankeyNode struct {
	id    string
	index int
}

type sankeyLink struct {
	source, target *sankeyNode
	value          float64
}

type sankeyDb struct {
	nodes []*sankeyNode
	links []sankeyLink
	// title is "" where upstream's is undefined: every use treats the two alike. The parser
	// never sets accTitle or accDescr, so they are left out.
	title  string
	lookup map[string]*sankeyNode
}

func newSankeyDb() *sankeyDb {
	return &sankeyDb{lookup: map[string]*sankeyNode{}}
}

func (db *sankeyDb) findOrCreateNode(id string) *sankeyNode {
	node, ok := db.lookup[id]
	if !ok {
		node = &sankeyNode{id: id, index: len(db.nodes)}
		db.lookup[id] = node
		db.nodes = append(db.nodes, node)
	}
	return node
}

func (db *sankeyDb) addLink(source, target *sankeyNode, value float64) {
	db.links = append(db.links, sankeyLink{source: source, target: target, value: value})
}
