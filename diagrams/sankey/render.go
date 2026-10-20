package sankey

import (
	"math"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/svg"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

const (
	labelGap = 6
	valueGap = 15
)

func numberOption(conf *jsobj.Object, key string, fallback, least, most float64) float64 {
	v, _ := conf.Get(key)
	if value, ok := v.(float64); ok && value >= least {
		return min(value, most)
	}
	return fallback
}

func textOption(conf *jsobj.Object, key string) string {
	switch v, _ := conf.Get(key); value := v.(type) {
	case string:
		return value
	case float64:
		if isFinite(value) {
			return jsnum.Format(value)
		}
	}
	return ""
}

// rounded keeps two decimals, as Mermaid shows values; numbers too large for that are left
// as they are.
func rounded(value float64) string {
	if math.Abs(value) < 1e15 {
		value = jsmath.Round(value*100) / 100
	}
	return jsnum.Format(value)
}

// fillStyle is a colour from the configuration, as an inline style that overrides the series
// colour.
func fillStyle(color any) string {
	if s, ok := color.(string); ok {
		return svg.ResolveStyle([]string{"fill:" + s}).Shape
	}
	return ""
}

func renderSankey(model *sankeyDb, config *jsobj.Object, rc *core.Context) core.Rendered {
	section, _ := config.Get("sankey")
	conf, _ := section.(*jsobj.Object)
	return common.FitWidth(rc.Options, numberOption(conf, "width", 600, 1, 10000), 160, func(chartWidth float64, _ bool) core.Rendered {
		return draw(model, conf, rc, chartWidth)
	})
}

func draw(model *sankeyDb, conf *jsobj.Object, rc *core.Context, chartWidth float64) core.Rendered {
	options := rc.Options
	size := 16.0
	if options.FontSize != nil {
		size = *options.FontSize
	}
	small := jsmath.Round(size * 0.875)
	measurer := rc.Measurer()
	pad := 8.0
	if options.Padding != nil {
		pad = *options.Padding
	}

	chartHeight := numberOption(conf, "height", 400, 1, 10000)
	nodeWidth := min(numberOption(conf, "nodeWidth", 10, 1, 200), chartWidth)
	showValues, _ := conf.Get("showValues")
	showsValues := showValues != false
	prefix := textOption(conf, "prefix")
	suffix := textOption(conf, "suffix")
	labelStyle, _ := conf.Get("labelStyle")
	outlined := labelStyle == "outlined"
	align := alignment("justify")
	if a, _ := conf.Get("nodeAlignment"); a == "left" || a == "right" || a == "center" {
		align = alignment(a.(string))
	}
	linkColor := "gradient"
	if c, ok := conf.Get("linkColor"); ok {
		if s, isString := c.(string); isString {
			linkColor = s
		}
	}

	padding := numberOption(conf, "nodePadding", 12, 0, 200)
	if showsValues {
		padding += valueGap
	}
	nodes, links := layoutSankey(model, layoutOptions{
		width:       chartWidth,
		height:      chartHeight,
		nodeWidth:   nodeWidth,
		nodePadding: padding,
		labelHeight: small * 1.15,
		alignment:   align,
	})

	custom := map[string]string{}
	if colors, ok := conf.Get("nodeColors"); ok {
		if obj, isObj := colors.(*jsobj.Object); isObj {
			for _, id := range obj.Keys() {
				color, _ := obj.Get(id)
				if style := fillStyle(color); style != "" {
					custom[id] = style
				}
			}
		}
	}
	nodeStyle := func(i int) string { return custom[model.nodes[i].id] }

	// Labels sit beside the bars, on the side that faces the middle of the chart.
	central := 0
	most := 0.0
	for _, node := range nodes {
		if node.value > most {
			most = node.value
			central = node.layer
		}
	}
	minX, maxX := 0.0, chartWidth
	minY, maxY := 0.0, chartHeight
	var labelsOut, nodesOut strings.Builder
	for i, node := range nodes {
		id := model.nodes[i].id
		height := max(node.height, 1)
		nodesOut.WriteString(`<rect class="pele-sankey-node" data-id="` + svg.EscText(id) + `" x="` + svg.Num(node.x) + `" y="` + svg.Num(node.y) + `" width="` +
			svg.Num(nodeWidth) + `" height="` + svg.Num(height) + `" rx="` + svg.Num(min(2, height/2)) + `" fill="` + svg.SeriesColor(i) + `"` + nodeStyle(i) + `/>`)

		value := ""
		if showsValues {
			value = prefix + rounded(node.value) + suffix
		}
		var after bool
		if outlined {
			after = node.layer >= central
		} else {
			after = node.x < chartWidth/2
		}
		measured := id
		if value != "" {
			measured = id + " " + value
		}
		textWidth := measurer.Width(measured, small, 0)
		x := node.x - labelGap
		if after {
			x = node.x + nodeWidth + labelGap
		}
		y := node.y + float64(node.height/2)
		if after {
			minX = min(minX, x)
			maxX = max(maxX, x+textWidth)
		} else {
			minX = min(minX, x-textWidth)
			maxX = max(maxX, x)
		}
		minY = min(minY, y-float64(small*0.75))
		maxY = max(maxY, y+float64(small*0.75))
		anchor := "end"
		if after {
			anchor = "start"
		}
		position := ` x="` + svg.Num(x) + `" y="` + svg.Num(y+float64(small*0.35)) + `" text-anchor="` + anchor + `"`
		body := svg.Esc(id)
		if value != "" {
			body += ` <tspan class="pele-sankey-value" fill="var(--_m)">` + svg.Esc(value) + `</tspan>`
		}
		labelsOut.WriteString(`<g class="pele-label" data-id="` + svg.EscText(id) + `">`)
		if outlined {
			plain := svg.Esc(id)
			if value != "" {
				plain += " " + svg.Esc(value)
			}
			labelsOut.WriteString(`<text` + position + ` fill="none" stroke="var(--_bg)" stroke-width="3" stroke-linejoin="round">` + plain + `</text>`)
		}
		labelsOut.WriteString(`<text` + position + `>` + body + `</text></g>`)
	}

	fixed := ""
	if linkColor != "gradient" && linkColor != "source" && linkColor != "target" {
		fixed = fillStyle(linkColor)
	}
	var linksOut strings.Builder
	for _, link := range links {
		// Without definitions there are no gradients, so a gradient link takes its source's colour.
		end := link.source
		if linkColor == "target" {
			end = link.target
		}
		half := float64(max(link.thickness, 1) / 2)
		x0 := nodes[link.source].x + nodeWidth
		x1 := nodes[link.target].x
		mid := svg.Num((x0 + x1) / 2)
		from := svg.EscText(model.nodes[link.source].id)
		to := svg.EscText(model.nodes[link.target].id)
		style := fixed
		if style == "" {
			style = nodeStyle(end)
		}
		linksOut.WriteString(`<path class="pele-sankey-link" data-id="` + from + `-&gt;` + to + `" d="M` + svg.Num(x0) + `,` + svg.Num(link.sourceY-half) +
			`C` + mid + `,` + svg.Num(link.sourceY-half) + ` ` + mid + `,` + svg.Num(link.targetY-half) + ` ` + svg.Num(x1) + `,` + svg.Num(link.targetY-half) +
			`L` + svg.Num(x1) + `,` + svg.Num(link.targetY+half) +
			`C` + mid + `,` + svg.Num(link.targetY+half) + ` ` + mid + `,` + svg.Num(link.sourceY+half) + ` ` + svg.Num(x0) + `,` + svg.Num(link.sourceY+half) +
			`Z" fill="` + svg.SeriesColor(end) + `"` + style + `>` +
			`<title>` + from + ` → ` + to + `: ` + svg.EscText(prefix) + rounded(link.value) + svg.EscText(suffix) + `</title></path>`)
	}

	title := text.Layout(model.title, false, measurer, size, common.TitleRoom(options), text.Bold, rc.Icons)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	inner := max(maxX-minX, title.Width)
	ox := pad - minX + float64((inner-(maxX-minX))/2)
	oy := pad + titleHeight - minY
	width := math.Ceil(inner + float64(2*pad))
	height := math.Ceil(maxY - minY + titleHeight + float64(2*pad))

	content := svg.LabelSVG(title, width/2, pad+float64(title.Height/2), ` class="pele-title" font-weight="var(--_tw)"`, rc.Icons) +
		`<g transform="translate(` + svg.Num(ox) + `,` + svg.Num(oy) + `)">` +
		`<g class="pele-sankey-links" fill-opacity="0.4">` + linksOut.String() + `</g>` +
		`<g class="pele-sankey-nodes">` + nodesOut.String() + `</g>` +
		`<g class="pele-labels" font-size="` + jsnum.Format(small) + `">` + labelsOut.String() + `</g>` +
		`</g>`
	doc := svg.Document("sankey", width, height, size, svg.DocumentOptions{
		IDPrefix:       options.IDPrefix,
		Responsive:     rc.Responsive,
		FontFamily:     options.FontFamily,
		FontFamilyMono: options.FontFamilyMono,
	}, svg.Accessible{}, content)
	return core.Rendered{SVG: doc, Width: width, Height: height, Links: []core.Link{}}
}
