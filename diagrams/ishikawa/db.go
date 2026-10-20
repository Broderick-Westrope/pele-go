package ishikawa

type frame struct {
	level int
	node  *ishikawaNode
}

// ishikawaDb is the model: the root is the effect; its children are the categories of
// causes, theirs the causes, and so on.
type ishikawaDb struct {
	title *string
	root  *ishikawaNode
	// stack is the line of ancestors of the last node, each with its level.
	stack     []frame
	baseLevel *int
}

// addNode adds a line. The first line is the effect. Every later line is a cause, nested by
// its indentation relative to the first cause, so the effect itself may be indented more
// than the causes.
func (db *ishikawaDb) addNode(rawLevel int, text string) {
	node := &ishikawaNode{text: text}
	if db.root == nil {
		db.root = node
		db.stack = []frame{{level: 0, node: node}}
		return
	}
	if db.baseLevel == nil {
		db.baseLevel = &rawLevel
	}
	level := max(rawLevel-*db.baseLevel+1, 1)
	for len(db.stack) > 1 && db.stack[len(db.stack)-1].level >= level {
		db.stack = db.stack[:len(db.stack)-1]
	}
	parent := db.stack[len(db.stack)-1].node
	parent.children = append(parent.children, node)
	db.stack = append(db.stack, frame{level: level, node: node})
}
