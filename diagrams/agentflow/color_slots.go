package agentflow

// Two rules, as in Mermaid: a node takes the color of its kind, from a fixed slot, so editing
// the diagram around it never recolors it; containers count through the slots above the
// kinds, in the order they were declared, so a frame does not take the color of a node inside
// it.

var kindSlot = map[string]int{
	"tool":      0,
	"task":      1,
	"decision":  2,
	"input":     3,
	"refdoc":    4,
	"connector": 5,
	"action":    6,
}

const kindCount = 7

// containerSlotCount is at least one, so that a palette shorter than the kinds still gives
// containers a slot.
func containerSlotCount(paletteLength int) int {
	return max(1, paletteLength-kindCount)
}

func containerSlot(n, paletteLength int) int {
	slot := kindCount + n%containerSlotCount(paletteLength)
	if paletteLength > 0 {
		return slot % paletteLength
	}
	return slot
}

func assignColorSlots(nodes []*graphNode, kindOf func(id string) string, containerOrder map[string]int, paletteLength int) {
	fallback := len(containerOrder)
	for _, node := range nodes {
		// A collapsed container is drawn as a node but keeps its slot, so collapsing one does
		// not recolor the containers after it.
		if node.isGroup || node.shape == "collapsedGroup" {
			n, ok := containerOrder[node.id]
			if !ok {
				n = fallback
				fallback++
			}
			node.colorIndex = containerSlot(n, paletteLength)
			continue
		}
		if kind := kindOf(node.id); kind != "" {
			if _, ok := kindSlot[kind]; ok {
				node.kind = kind
			}
		}
	}
}
