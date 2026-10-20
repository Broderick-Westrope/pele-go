package pie

import (
	"math"
	"strconv"

	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/svg"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

const (
	radiusDefault = 120
	swatch        = 12
	row           = 26
	gapX          = 32
	outsideMargin = 28
)

func option(config *jsobj.Object, key string) (any, bool) {
	section, _ := config.Get("pie")
	obj, ok := section.(*jsobj.Object)
	if !ok {
		return nil, false
	}
	return obj.Get(key)
}

func point(angle, radius float64) string {
	return svg.Num(jsmath.Sin(angle)*radius) + "," + svg.Num(-jsmath.Cos(angle)*radius)
}

func slice(start, end, outer, inner float64) string {
	if end-start >= float64(math.Pi*2)-1e-9 {
		ring := func(r float64, sweep string) string {
			return "M0," + svg.Num(-r) + "A" + svg.Num(r) + "," + svg.Num(r) + " 0 1 " + sweep + " 0," + svg.Num(r) +
				"A" + svg.Num(r) + "," + svg.Num(r) + " 0 1 " + sweep + " 0," + svg.Num(-r) + "Z"
		}
		out := ring(outer, "1")
		if inner > 0 {
			out += ring(inner, "0")
		}
		return out
	}
	large := "0"
	if end-start > math.Pi {
		large = "1"
	}
	if inner <= 0 {
		return "M0,0L" + point(start, outer) + "A" + svg.Num(outer) + "," + svg.Num(outer) + " 0 " + large + " 1 " + point(end, outer) + "Z"
	}
	return "M" + point(start, outer) + "A" + svg.Num(outer) + "," + svg.Num(outer) + " 0 " + large + " 1 " + point(end, outer) +
		"L" + point(end, inner) + "A" + svg.Num(inner) + "," + svg.Num(inner) + " 0 " + large + " 0 " + point(start, inner) + "Z"
}

// toFixed0 is x.toFixed(0) for the percentages here, which are never negative or as large
// as 1e21: the nearest integer, the larger of two.
func toFixed0(x float64) string {
	return jsnum.Format(jsmath.Round(x))
}

func renderPie(m model, config *jsobj.Object, rc *core.Context) core.Rendered {
	return common.FitWidth(rc.Options, radiusDefault*2, 140, func(diameter float64, tight bool) core.Rendered {
		return draw(m, config, rc, diameter/2, tight)
	})
}

type placedLabel struct{ x, y, width float64 }

// draw's tight is set when the chart does not fit the width it has with its legend beside it.
func draw(m model, config *jsobj.Object, rc *core.Context, radius float64, tight bool) core.Rendered {
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

	textPosition := 0.75
	if v, ok := option(config, "textPosition"); ok {
		if p, isNum := v.(float64); isNum && p >= 0 && p <= 1 {
			textPosition = p
		}
	}
	inner := 0.0
	if v, ok := option(config, "donutHole"); ok {
		if h, isNum := v.(float64); isNum && h > 0 && h <= 0.9 {
			inner = h * radius
		}
	}
	placed := "right"
	if v, ok := option(config, "legendPosition"); ok && v != nil {
		placed = jsobj.ToString(v)
	}
	legendPosition := placed
	if tight && (placed == "left" || placed == "right") {
		legendPosition = "bottom"
	}

	var labelsOf []string
	var values []float64
	for label, value := range m.sections.All() {
		labelsOf = append(labelsOf, label)
		values = append(values, value)
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}

	legend := make([]text.Label, len(labelsOf))
	for i, label := range labelsOf {
		s := label
		if m.showData {
			s = label + " [" + jsnum.Format(values[i]) + "]"
		}
		legend[i] = text.Layout(s, false, measurer, small, 4000, 0, rc.Icons)
	}
	legendWidth := 0.0
	for _, label := range legend {
		legendWidth = max(legendWidth, label.Width)
	}
	if len(legend) > 0 {
		legendWidth += swatch + 8
	}
	legendHeight := float64(len(legend) * row)

	title := text.Layout(m.title, false, measurer, size, common.TitleRoom(options), text.Bold, rc.Icons)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 16
	}

	// A slice too thin for its percentage gets the label outside the rim, which needs a margin.
	shares := make([]float64, len(values))
	for i, value := range values {
		share := 0.0
		if sum > 0 {
			share = value / sum
		}
		if share*100 >= 1 && toFixed0(share*100) != "0" {
			shares[i] = share
		}
	}
	narrow := make([]bool, len(shares))
	margin := 0.0
	for i, share := range shares {
		if share == 0 || share >= 0.5 {
			continue
		}
		chord := 2 * radius * textPosition * jsmath.Sin(share*math.Pi)
		narrow[i] = chord < measurer.Width(toFixed0(share*100)+"%", small, 0)+6
		if narrow[i] {
			margin = outsideMargin
		}
	}

	side := ""
	if legendPosition == "left" || legendPosition == "right" || legendPosition == "center" {
		side = legendPosition
	}
	stacked := legendPosition == "top" || legendPosition == "bottom"
	diameter := (radius + margin) * 2
	width := diameter
	height := diameter
	if side == "left" || side == "right" {
		if legendWidth > 0 {
			width += gapX + legendWidth
		}
		height = max(height, legendHeight)
	} else if stacked {
		width = max(width, legendWidth)
		if legendHeight > 0 {
			height += 20 + legendHeight
		}
	}
	width = max(width, title.Width)

	var cx float64
	switch side {
	case "left":
		cx = pad + (legendWidth + gapX + float64(diameter/2))
	case "right":
		cx = pad + float64(diameter/2)
	default:
		cx = pad + float64(width/2)
	}
	bodyTop := pad + titleHeight
	var cy float64
	switch {
	case legendPosition == "top":
		cy = bodyTop + (legendHeight + 20 + float64(diameter/2))
	case stacked:
		cy = bodyTop + float64(diameter/2)
	default:
		cy = bodyTop + float64(height/2)
	}

	slices, labels, outside := "", "", ""
	angle := 0.0
	rim := radius + outsideMargin/2
	var placedOutside []placedLabel
	clear := func(x, y, width float64) bool {
		for _, other := range placedOutside {
			if !(math.Abs(x-other.x) >= float64((width+other.width)/2)+2 || math.Abs(y-other.y) >= small+2) {
				return false
			}
		}
		return true
	}
	for index, label := range labelsOf {
		share := shares[index]
		if share == 0 {
			continue
		}
		end := angle + float64(float64(share*math.Pi)*2)
		slices += `<path class="pele-slice" data-id="` + svg.EscText(label) + `" d="` + slice(angle, end, radius, inner) + `" fill="` + svg.SeriesColor(index) + `"/>`
		mid := (angle + end) / 2
		pct := toFixed0(share*100) + "%"
		if narrow[index] {
			width := measurer.Width(pct, small, 0)
			at := mid
			for step := 0; step < 360 && !clear(float64(jsmath.Sin(at)*rim), float64(-jsmath.Cos(at)*rim), width); step++ {
				at += 2 / rim
			}
			x := jsmath.Sin(at) * rim
			y := float64(-jsmath.Cos(at) * rim)
			placedOutside = append(placedOutside, placedLabel{x, y, width})
			outside += `<text x="` + svg.Num(x) + `" y="` + svg.Num(y+float64(small*0.35)) + `">` + pct + `</text>`
		} else {
			r := radius * textPosition
			labels += `<text x="` + svg.Num(jsmath.Sin(mid)*r) + `" y="` + svg.Num(float64(-jsmath.Cos(mid)*r)+float64(small*0.35)) + `">` + pct + `</text>`
		}
		angle = end
	}
	if slices == "" {
		slices = `<circle class="pele-slice" r="` + jsnum.Format(radius) + `" fill="none" stroke="var(--_b)"/>`
	}

	var lx, ly float64
	switch side {
	case "right":
		lx = cx + float64(diameter/2) + gapX
		ly = bodyTop + float64((height-legendHeight)/2)
	case "left":
		lx = pad
		ly = bodyTop + float64((height-legendHeight)/2)
	case "center":
		lx = cx - float64(legendWidth/2)
		ly = cy - float64(legendHeight/2)
	default:
		lx = pad + float64((width-legendWidth)/2)
		if legendPosition == "top" {
			ly = bodyTop
		} else {
			ly = bodyTop + diameter + 20
		}
	}
	legendOut := ""
	for index, label := range legend {
		y := ly + float64(float64(index)*row) + row/2
		legendOut += `<g class="pele-legend-item" data-id="` + svg.EscText(labelsOf[index]) + `">` +
			`<rect x="` + svg.Num(lx) + `" y="` + svg.Num(y-swatch/2) + `" width="` + strconv.Itoa(swatch) + `" height="` + strconv.Itoa(swatch) +
			`" rx="2" fill="` + svg.SeriesColor(index) + `"/>` +
			svg.LabelSVG(label, lx+swatch+8+float64(label.Width/2), y, "", nil) +
			"</g>"
	}

	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(height + titleHeight + 2*pad)
	smallAttr := jsnum.Format(small)
	translate := `transform="translate(` + svg.Num(cx) + "," + svg.Num(cy) + `)"`
	content := svg.LabelSVG(title, totalWidth/2, pad+float64(title.Height/2), ` class="pele-title" font-weight="var(--_tw)"`, nil) +
		`<g class="pele-slices" ` + translate + ` stroke="var(--_bg)" stroke-width="1" stroke-linejoin="round">` + slices + `</g>`
	if labels != "" {
		content += `<g class="pele-slice-labels" ` + translate + ` font-size="` + smallAttr + `" text-anchor="middle" fill="var(--_bg)">` + labels + `</g>`
	}
	if outside != "" {
		content += `<g class="pele-slice-labels" ` + translate + ` font-size="` + smallAttr + `" text-anchor="middle">` + outside + `</g>`
	}
	if legendOut != "" {
		content += `<g class="pele-legend" font-size="` + smallAttr + `">` + legendOut + `</g>`
	}

	doc := svg.Document("pie", totalWidth, totalHeight, size, svg.DocumentOptions{
		IDPrefix:       options.IDPrefix,
		Responsive:     rc.Responsive,
		FontFamily:     options.FontFamily,
		FontFamilyMono: options.FontFamilyMono,
	}, svg.Accessible{AccTitle: m.accTitle, AccDescr: m.accDescr}, content)
	return core.Rendered{SVG: doc, Width: totalWidth, Height: totalHeight, Links: []core.Link{}}
}
