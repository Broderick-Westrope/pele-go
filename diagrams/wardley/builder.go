package wardley

import (
	"github.com/Broderick-Westrope/pele-go/internal/omap"
)

// Flows of a link.
const (
	flowNone = iota
	flowForward
	flowBackward
	flowBidirectional
)

// wardleyNode is a placed node. Coordinates are percentages: x is evolution, y is
// visibility. Upstream checks at build time that every node has them; every node added has,
// so the check is left out.
type wardleyNode struct {
	id    string
	label string
	x, y  float64
	// className is "anchor", "component" or "pipeline-component".
	className        string
	labelOffsetX     *float64
	labelOffsetY     *float64
	inPipeline       bool
	isPipelineParent bool
	inertia          bool
	// sourceStrategy is "build", "buy", "outsource", "market", or "" for none.
	sourceStrategy string
}

type wardleyLink struct {
	source, target string
	dashed         bool
	label          string
	flow           int
}

type wardleyTrend struct {
	nodeID           string
	targetX, targetY float64
}

type wardleyPipeline struct {
	nodeID       string
	componentIDs []string
}

type wardleyAnnotation struct {
	number      float64
	coordinates [][2]float64
	text        string
}

type wardleyNote struct {
	text string
	x, y float64
}

type wardleyForce struct {
	name string
	x, y float64
}

// wardleyAxesConfig leaves out upstream's xLabel and yLabel, which nothing sets.
type wardleyAxesConfig struct {
	stages []string
	// stageBoundaries is the right edge of each stage, from 0 to 1.
	stageBoundaries []float64
}

type wardleyBuildResult struct {
	nodes          []*wardleyNode
	links          []wardleyLink
	trends         []wardleyTrend
	pipelines      []*wardleyPipeline
	annotations    []wardleyAnnotation
	notes          []wardleyNote
	accelerators   []wardleyForce
	deaccelerators []wardleyForce
	annotationsBox *[2]float64
	axes           wardleyAxesConfig
	size           *sizeAst
}

// nodeUpdate is a node given to addNode. A nil field is one the update does not set: a
// node added again keeps its class and label offsets unless the new one sets them, and
// keeps its inertia and strategy when the update has no such keys (an anchor).
type nodeUpdate struct {
	id, label      string
	x, y           float64
	className      string
	labelOffsetX   *float64
	labelOffsetY   *float64
	inertia        *bool
	sourceStrategy *string
}

type wardleyBuilder struct {
	nodes          omap.Map[string, *wardleyNode]
	labels         *omap.Map[string, string]
	links          []wardleyLink
	trends         omap.Map[string, wardleyTrend]
	pipelines      omap.Map[string, *wardleyPipeline]
	annotations    []wardleyAnnotation
	notes          []wardleyNote
	accelerators   []wardleyForce
	deaccelerators []wardleyForce
	annotationsBox *[2]float64
	axes           wardleyAxesConfig
	size           *sizeAst
}

func (b *wardleyBuilder) addNode(n nodeUpdate) {
	node := &wardleyNode{id: n.id, label: n.label}
	if existing, ok := b.nodes.Get(n.id); ok {
		copied := *existing
		node = &copied
	}
	node.id = n.id
	node.label = n.label
	node.x = n.x
	node.y = n.y
	if n.className != "" {
		node.className = n.className
	}
	if n.labelOffsetX != nil {
		node.labelOffsetX = n.labelOffsetX
	}
	if n.labelOffsetY != nil {
		node.labelOffsetY = n.labelOffsetY
	}
	if n.inertia != nil {
		node.inertia = *n.inertia
	}
	if n.sourceStrategy != nil {
		node.sourceStrategy = *n.sourceStrategy
	}
	b.nodes.Set(n.id, node)
	b.labels = nil
}

func (b *wardleyBuilder) addLink(link wardleyLink) {
	b.links = append(b.links, link)
}

func (b *wardleyBuilder) addTrend(trend wardleyTrend) {
	b.trends.Set(trend.nodeID, trend)
}

func (b *wardleyBuilder) startPipeline(nodeID string) {
	b.pipelines.Set(nodeID, &wardleyPipeline{nodeID: nodeID})
	if node, ok := b.nodes.Get(nodeID); ok {
		node.isPipelineParent = true
	}
}

func (b *wardleyBuilder) addPipelineComponent(pipelineNodeID, componentID string) {
	if p, ok := b.pipelines.Get(pipelineNodeID); ok {
		p.componentIDs = append(p.componentIDs, componentID)
	}
	if node, ok := b.nodes.Get(componentID); ok {
		node.inPipeline = true
	}
}

func (b *wardleyBuilder) addAnnotation(annotation wardleyAnnotation) {
	b.annotations = append(b.annotations, annotation)
}

func (b *wardleyBuilder) addNote(note wardleyNote) {
	b.notes = append(b.notes, note)
}

func (b *wardleyBuilder) addAccelerator(accelerator wardleyForce) {
	b.accelerators = append(b.accelerators, accelerator)
}

func (b *wardleyBuilder) addDeaccelerator(deaccelerator wardleyForce) {
	b.deaccelerators = append(b.deaccelerators, deaccelerator)
}

func (b *wardleyBuilder) setAnnotationsBox(x, y float64) {
	b.annotationsBox = &[2]float64{x, y}
}

func (b *wardleyBuilder) setSize(width, height float64) {
	b.size = &sizeAst{width, height}
}

func (b *wardleyBuilder) getNode(id string) (*wardleyNode, bool) {
	return b.nodes.Get(id)
}

// resolveNodeID returns name when it is a node id, or else the id of the first node with
// name as its label. Pipeline components have ids like "Parent_Child" and are linked by label.
func (b *wardleyBuilder) resolveNodeID(name string) string {
	if b.nodes.Has(name) {
		return name
	}
	if b.labels == nil {
		b.labels = &omap.Map[string, string]{}
		for id, node := range b.nodes.All() {
			if !b.labels.Has(node.label) {
				b.labels.Set(node.label, id)
			}
		}
	}
	if id, ok := b.labels.Get(name); ok {
		return id
	}
	return name
}

func (b *wardleyBuilder) build() wardleyBuildResult {
	var out wardleyBuildResult
	for _, node := range b.nodes.All() {
		out.nodes = append(out.nodes, node)
	}
	out.links = b.links
	for _, t := range b.trends.All() {
		out.trends = append(out.trends, t)
	}
	for _, p := range b.pipelines.All() {
		out.pipelines = append(out.pipelines, p)
	}
	out.annotations = b.annotations
	out.notes = b.notes
	out.accelerators = b.accelerators
	out.deaccelerators = b.deaccelerators
	out.annotationsBox = b.annotationsBox
	out.axes = b.axes
	out.size = b.size
	return out
}
