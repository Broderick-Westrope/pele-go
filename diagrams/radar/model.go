package radar

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type radarAxis struct {
	name  string
	label string
}

type radarCurve struct {
	name    string
	label   string
	entries []float64
}

type radarOptions struct {
	showLegend bool
	ticks      float64
	// max is nil where upstream's is null.
	max       *float64
	min       float64
	graticule string
}

type radarModel struct {
	title, accTitle, accDescr string
	axes                      []radarAxis
	curves                    []radarCurve
	options                   radarOptions
}

const maxTicks = 32

func invalid(message string) *perr.Error {
	e := perr.New(message, perr.Semantic)
	e.Type = "radar"
	return e
}

func buildRadar(ast radarAst, title *string) *radarModel {
	axes := make([]radarAxis, len(ast.axes))
	for i, a := range ast.axes {
		axes[i] = radarAxis{name: a.name, label: a.name}
		if a.label != nil {
			axes[i].label = *a.label
		}
	}

	curves := make([]radarCurve, len(ast.curves))
	for i, c := range ast.curves {
		var entries []float64
		if c.entries[0].axis == nil {
			for _, e := range c.entries {
				entries = append(entries, e.value)
			}
		} else {
			if len(axes) == 0 {
				perr.Throw(invalid("Axes must be populated before curves for reference entries"))
			}
			byAxis := map[string]float64{}
			for _, e := range c.entries {
				if e.axis == nil {
					continue
				}
				if _, seen := byAxis[*e.axis]; !seen {
					byAxis[*e.axis] = e.value
				}
			}
			for _, a := range axes {
				value, ok := byAxis[a.name]
				if !ok {
					perr.Throw(invalid("Missing entry for axis " + a.label))
				}
				entries = append(entries, value)
			}
		}
		curves[i] = radarCurve{name: c.name, label: c.name, entries: entries}
		if c.label != nil {
			curves[i].label = *c.label
		}
	}

	options := radarOptions{showLegend: true, ticks: 5, min: 0, graticule: "circle"}
	for _, o := range ast.options {
		switch o.name {
		case "showLegend":
			options.showLegend = o.boolean
		case "graticule":
			options.graticule = o.text
		case "ticks":
			options.ticks = o.number
		case "max":
			v := o.number
			options.max = &v
		case "min":
			options.min = o.number
		}
	}
	if options.ticks > maxTicks {
		options.ticks = maxTicks
	}

	m := &radarModel{axes: axes, curves: curves, options: options}
	if ast.title != nil && *ast.title != "" {
		m.title = *ast.title
	} else if title != nil {
		m.title = *title
	}
	if ast.accTitle != nil {
		m.accTitle = jsstr.TrimStart(*ast.accTitle)
	}
	if ast.accDescr != nil {
		m.accDescr = common.CollapseIndentedBreaks(*ast.accDescr)
	}
	return m
}
