package wardley

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type wardleyModel struct {
	wardleyBuildResult
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
}

func fail(message string) {
	e := perr.New(message, perr.Semantic)
	e.Type = "wardley"
	perr.Throw(e)
}

// toPercent reads values up to 1 as fractions; larger ones are already percentages.
func toPercent(value float64, context string) float64 {
	normalized := value
	if value <= 1 {
		normalized = value * 100
	}
	if normalized < 0 || normalized > 100 {
		fail(context + " must be between 0-1 (decimal) or 0-100 (percentage). Received: " + jsnum.Format(value))
	}
	return normalized
}

func toCoordinates(visibility, evolution float64, context string) (x, y float64) {
	x = toPercent(evolution, context+" evolution")
	y = toPercent(visibility, context+" visibility")
	return x, y
}

func flowFromPort(port string) int {
	switch port {
	case "+<>":
		return flowBidirectional
	case "+<":
		return flowBackward
	case "+>":
		return flowForward
	}
	return flowNone
}

func flowFromArrow(arrow *string) (flow int, label *string) {
	if arrow == nil || !strings.HasPrefix(*arrow, "+") {
		return flowNone, nil
	}
	a := *arrow
	if len(a) > 1 && a[1] == '\'' {
		if end := strings.Index(a[2:], "'"); end != -1 {
			label = ptr(a[2 : 2+end])
		}
	}
	switch {
	case strings.Contains(a, "<>"):
		return flowBidirectional, label
	case strings.Contains(a, "<"):
		return flowBackward, label
	case strings.Contains(a, ">"):
		return flowForward, label
	}
	return flowNone, label
}

func offsets(label *labelAst) (*float64, *float64) {
	if label == nil {
		return nil, nil
	}
	sign := func(neg bool) float64 {
		if neg {
			return -1
		}
		return 1
	}
	return ptr(sign(label.negX) * label.offsetX), ptr(sign(label.negY) * label.offsetY)
}

// populate is Mermaid's populateDb: it fills the builder from the syntax tree, in Mermaid's
// order.
func populate(ast wardleyAst, db *wardleyBuilder) {
	if ast.size != nil {
		db.setSize(ast.size.width, ast.size.height)
	}

	if ast.evolution != nil {
		var axes wardleyAxesConfig
		for _, stage := range ast.evolution {
			if stage.secondName != nil {
				axes.stages = append(axes.stages, jsstr.Trim(stage.name)+" / "+jsstr.Trim(*stage.secondName))
			} else {
				axes.stages = append(axes.stages, jsstr.Trim(stage.name))
			}
			if stage.boundary != nil {
				axes.stageBoundaries = append(axes.stageBoundaries, *stage.boundary)
			}
		}
		db.axes = axes
	}

	for _, anchor := range ast.anchors {
		x, y := toCoordinates(anchor.visibility, anchor.evolution, `Anchor "`+anchor.name+`"`)
		db.addNode(nodeUpdate{id: anchor.name, label: anchor.name, x: x, y: y, className: "anchor"})
	}

	for _, component := range ast.components {
		x, y := toCoordinates(component.visibility, component.evolution, `Component "`+component.name+`"`)
		labelOffsetX, labelOffsetY := offsets(component.label)
		db.addNode(nodeUpdate{
			id:             component.name,
			label:          component.name,
			x:              x,
			y:              y,
			className:      "component",
			labelOffsetX:   labelOffsetX,
			labelOffsetY:   labelOffsetY,
			inertia:        ptr(component.inertia),
			sourceStrategy: ptr(component.strategy),
		})
	}

	for _, note := range ast.notes {
		x, y := toCoordinates(note.visibility, note.evolution, `Note "`+note.text+`"`)
		db.addNote(wardleyNote{note.text, x, y})
	}

	for _, pipeline := range ast.pipelines {
		parent, ok := db.getNode(pipeline.parent)
		if !ok {
			fail(`Pipeline "` + pipeline.parent + `" must reference an existing component with coordinates.`)
		}
		y := parent.y
		db.startPipeline(pipeline.parent)
		for _, component := range pipeline.components {
			id := pipeline.parent + "_" + component.name
			labelOffsetX, labelOffsetY := offsets(component.label)
			x := toPercent(component.evolution, `Pipeline component "`+component.name+`" evolution`)
			db.addNode(nodeUpdate{
				id:             id,
				label:          component.name,
				x:              x,
				y:              y,
				className:      "pipeline-component",
				labelOffsetX:   labelOffsetX,
				labelOffsetY:   labelOffsetY,
				inertia:        ptr(false),
				sourceStrategy: ptr(""),
			})
			db.addPipelineComponent(pipeline.parent, id)
		}
	}

	for _, link := range ast.links {
		dashed := link.arrow != nil && (strings.Contains(*link.arrow, "-.->") || strings.Contains(*link.arrow, ".-."))
		arrowFlow, arrowLabel := flowFromArrow(link.arrow)
		flow := flowFromPort(link.fromPort)
		if flow == flowNone {
			flow = flowFromPort(link.toPort)
		}
		if flow == flowNone {
			flow = arrowFlow
		}
		label := ""
		if arrowLabel != nil {
			label = *arrowLabel
		} else if link.linkLabel != nil {
			label = *link.linkLabel
		}
		db.addLink(wardleyLink{
			source: db.resolveNodeID(link.from),
			target: db.resolveNodeID(link.to),
			dashed: dashed,
			label:  label,
			flow:   flow,
		})
	}

	for _, evolve := range ast.evolves {
		if node, ok := db.getNode(evolve.component); ok {
			db.addTrend(wardleyTrend{
				nodeID:  evolve.component,
				targetX: toPercent(evolve.target, `Evolve target for "`+evolve.component+`"`),
				targetY: node.y,
			})
		}
	}

	if len(ast.annotations) > 0 {
		x, y := toCoordinates(ast.annotations[0][0], ast.annotations[0][1], "Annotations box")
		db.setAnnotationsBox(x, y)
	}

	for _, annotation := range ast.annotation {
		x, y := toCoordinates(annotation.x, annotation.y, "Annotation "+jsnum.Format(annotation.number))
		db.addAnnotation(wardleyAnnotation{number: annotation.number, coordinates: [][2]float64{{x, y}}, text: annotation.text})
	}

	for _, accelerator := range ast.accelerators {
		x, y := toCoordinates(accelerator.x, accelerator.y, `Accelerator "`+accelerator.name+`"`)
		db.addAccelerator(wardleyForce{accelerator.name, x, y})
	}

	for _, deaccelerator := range ast.deaccelerators {
		x, y := toCoordinates(deaccelerator.x, deaccelerator.y, `Deaccelerator "`+deaccelerator.name+`"`)
		db.addDeaccelerator(wardleyForce{deaccelerator.name, x, y})
	}
}

func buildWardley(ast wardleyAst, title *string) wardleyModel {
	db := &wardleyBuilder{}
	populate(ast, db)
	m := wardleyModel{wardleyBuildResult: db.build()}
	if ast.title != nil && *ast.title != "" {
		m.title = *ast.title
	} else if title != nil {
		m.title = *title
	}
	if ast.accTitle != nil && *ast.accTitle != "" {
		m.accTitle = jsstr.TrimStart(*ast.accTitle)
	}
	if ast.accDescr != nil && *ast.accDescr != "" {
		m.accDescr = collapseIndentedBreaks(*ast.accDescr)
	}
	return m
}

// collapseIndentedBreaks is s.replace(/\n\s+/g, '\n').
func collapseIndentedBreaks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\n' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		for j < len(s) {
			r, n := jsstr.Decode(s, j)
			if !jsstr.IsSpace(r) {
				break
			}
			j += n
		}
		b.WriteByte('\n')
		i = j
	}
	return b.String()
}
