package venn

import (
	"math"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/svg"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

const (
	radius    = 100
	maxGrowth = 1.6
)

type regionLine struct {
	// id is that of a text; hasID is false for the region's own label.
	id    string
	hasID bool
	raw   string
	size  float64
	base  text.Style
	label text.Label
}

type region struct {
	sets   []string
	inside []int
	spot   spot
	// What is written in the region: its label first, then its texts.
	lines []regionLine
	style svg.ResolvedStyle
}

// keyOf is one key for a list of set ids. Ids may hold any character, so they are not
// simply joined.
func keyOf(ids []string) string {
	if len(ids) == 1 {
		return ids[0]
	}
	return listKey(ids)
}

func renderVenn(model *vennDb, _ *jsobj.Object, rc *core.Context) core.Rendered {
	options := rc.Options
	size := 16.0
	if options.FontSize != nil {
		size = *options.FontSize
	}
	small := jsmath.Round(size * 0.875)
	tiny := jsmath.Round(size * 0.8125)
	measurer := rc.Measurer()
	pad := 8.0
	if options.Padding != nil {
		pad = *options.Padding
	}
	icons := rc.Icons

	circles := layoutCircles(model.subsets)
	index := map[string]int{}
	for i, c := range circles {
		index[c.id] = i
	}

	// Styles for the same targets add up, later declarations winning.
	var styles omap.Map[string, *omap.Map[string, string]]
	for _, declared := range model.styles {
		key := keyOf(declared.targets)
		merged, ok := styles.Get(key)
		if !ok {
			merged = &omap.Map[string, string]{}
			styles.Set(key, merged)
		}
		for property, value := range declared.styles.All() {
			merged.Set(property, value)
		}
	}
	styleOf := func(key string) svg.ResolvedStyle {
		declared, ok := styles.Get(key)
		if !ok {
			return svg.ResolvedStyle{}
		}
		var list []string
		for property, value := range declared.All() {
			list = append(list, property+":"+value)
		}
		return svg.ResolveStyle(list)
	}

	// Circles that touch each circle; only those can cut into a region of it.
	neighbors := make([][]int, len(circles))
	for a := range circles {
		rc.CheckCanceled()
		for b := a + 1; b < len(circles); b++ {
			if jsmath.Hypot(circles[a].x-circles[b].x, circles[a].y-circles[b].y) < circles[a].r+circles[b].r {
				neighbors[a] = append(neighbors[a], b)
				neighbors[b] = append(neighbors[b], a)
			}
		}
	}

	// One region for each set and each stated overlap; a later statement about the same sets
	// replaces an earlier one.
	var stated omap.Map[string, vennSubset]
	for _, subset := range model.subsets {
		stated.Set(keyOf(subset.sets), subset)
	}
	var regions omap.Map[string, *region]
	for key, subset := range stated.All() {
		var sets []string
		var within omap.Set[int]
		var inside []int
		seen := map[string]bool{}
		for _, id := range subset.sets {
			if !seen[id] {
				seen[id] = true
				sets = append(sets, id)
				inside = append(inside, index[id])
				within.Add(index[id])
			}
		}
		smallest := inside[0]
		for _, i := range inside {
			if circles[i].r < circles[smallest].r {
				smallest = i
			}
		}
		style := styleOf(key)
		var label string
		hasText := false
		if subset.label != nil {
			label, hasText = *subset.label, true
		} else if len(subset.sets) == 1 {
			label, hasText = subset.sets[0], true
		}
		var base text.Style
		if style.Bold {
			base |= text.Bold
		}
		if style.Italic {
			base |= text.Italic
		}
		var outside []int
		for _, i := range neighbors[smallest] {
			if !within.Has(i) {
				outside = append(outside, i)
			}
		}
		at := findSpot(circles, inside, outside)
		// A region that other circles cover, or all but cover, is written across them instead.
		if at.margin < float64(circles[smallest].r*0.2) {
			at = findSpot(circles, inside, nil)
		}
		r := &region{sets: sets, inside: inside, spot: at, style: style}
		if hasText && label != "" {
			fontSize := small
			if len(sets) == 1 {
				fontSize = size
			}
			r.lines = append(r.lines, regionLine{raw: label, size: fontSize, base: base, label: text.Layout(label, false, measurer, fontSize, 4000, base, icons)})
		}
		regions.Set(key, r)
	}
	for _, node := range model.textNodes {
		raw := node.id
		if node.label != nil {
			raw = *node.label
		}
		if r, ok := regions.Get(keyOf(node.sets)); ok {
			r.lines = append(r.lines, regionLine{id: node.id, hasID: true, raw: raw, size: tiny, label: text.Layout(raw, false, measurer, tiny, 4000, 0, icons)})
		}
	}

	// Lengths so far are in units of the layout. The scale gives the largest circle a set
	// radius, and grows, within limits, until every region holds what is written in it.
	largest := 0.0
	for _, c := range circles {
		if c.r > largest {
			largest = c.r
		}
	}
	scale := 1.0
	if largest > 0 {
		scale = radius / largest
	}
	growth := 1.0
	for _, r := range regions.All() {
		if r.spot.margin <= 0 {
			continue
		}
		width, height := 0.0, 0.0
		for _, line := range r.lines {
			width = max(width, line.label.Width)
			height += line.label.Height
		}
		need := float64(jsmath.Hypot(width, height)/2) + 4
		growth = max(growth, min(need/float64(r.spot.margin*scale), maxGrowth))
	}
	scale *= growth

	// What is still too wide for its region is wrapped to it.
	for _, r := range regions.All() {
		room := max(float64(r.spot.margin*scale)*1.8, 72)
		for k := range r.lines {
			line := &r.lines[k]
			if line.label.Width > room {
				line.label = text.Layout(line.raw, false, measurer, line.size, room, line.base, icons)
			}
		}
	}

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range circles {
		minX = min(minX, (c.x-c.r)*scale)
		minY = min(minY, (c.y-c.r)*scale)
		maxX = max(maxX, (c.x+c.r)*scale)
		maxY = max(maxY, (c.y+c.r)*scale)
	}
	for _, r := range regions.All() {
		wide, tall := 0.0, 0.0
		for _, line := range r.lines {
			wide = max(wide, line.label.Width)
			tall += line.label.Height
		}
		minX = min(minX, float64(r.spot.x*scale)-float64(wide/2))
		maxX = max(maxX, float64(r.spot.x*scale)+float64(wide/2))
		minY = min(minY, float64(r.spot.y*scale)-float64(tall/2))
		maxY = max(maxY, float64(r.spot.y*scale)+float64(tall/2))
	}
	if len(circles) == 0 {
		minX, minY, maxX, maxY = 0, 0, 0, 0
	}

	titleText := ""
	if model.title != "" {
		titleText = normalizeText(model.title)
	}
	title := text.Layout(titleText, false, measurer, size, 4000, text.Bold, icons)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	inner := max(maxX-minX, title.Width)
	ox := pad + float64((inner-(maxX-minX))/2) - minX
	oy := pad + titleHeight - minY
	width := math.Ceil(inner + float64(2*pad))
	height := math.Ceil(maxY - minY + titleHeight + float64(2*pad))
	placed := make([]circle, len(circles))
	for i, c := range circles {
		placed[i] = circle{id: c.id, x: float64(c.x*scale) + ox, y: float64(c.y*scale) + oy, r: c.r * scale}
	}

	var sets strings.Builder
	for i, c := range placed {
		color := svg.SeriesColor(i)
		var style svg.ResolvedStyle
		if r, ok := regions.Get(c.id); ok {
			style = r.style
		}
		sets.WriteString(`<circle class="pele-venn-set" data-id="` + svg.EscText(c.id) + `" cx="` + svg.Num(c.x) + `" cy="` + svg.Num(c.y) + `" r="` + svg.Num(c.r) +
			`" fill="` + color + `" fill-opacity="0.2" stroke="` + color + `"` + style.Shape + `/>`)
	}

	var overlaps, labels strings.Builder
	for _, r := range regions.All() {
		id := svg.EscText(strings.Join(r.sets, "|"))
		// An overlap is only drawn when a style gives it a look of its own; otherwise the circles show it.
		if len(r.sets) > 1 && r.style.Shape != "" {
			shared := make([]circle, len(r.inside))
			for k, i := range r.inside {
				shared[k] = placed[i]
			}
			if d := regionPath(shared, svg.Num); d != "" {
				overlaps.WriteString(`<path class="pele-venn-overlap" data-id="` + id + `" d="` + d + `" fill="none"` + r.style.Shape + `/>`)
			}
		}
		total := 0.0
		for _, line := range r.lines {
			total += line.label.Height
		}
		if total == 0 {
			continue
		}
		x := float64(r.spot.x*scale) + ox
		y := float64(r.spot.y*scale) + oy - float64(total/2)
		var content strings.Builder
		for _, line := range r.lines {
			var attrs string
			if !line.hasID {
				attrs = ` class="pele-label"`
				if line.size != size {
					attrs += ` font-size="` + jsnum.Format(line.size) + `"`
				}
				attrs += r.style.Text
			} else {
				attrs = ` class="pele-venn-text" data-id="` + svg.EscText(line.id) + `" font-size="` + jsnum.Format(tiny) + `" fill="var(--_m)"` + styleOf(line.id).Text
			}
			content.WriteString(svg.LabelSVG(line.label, x, y+float64(line.label.Height/2), attrs, icons))
			y += line.label.Height
		}
		labels.WriteString(`<g class="pele-venn-region" data-id="` + id + `">` + content.String() + `</g>`)
	}

	body := svg.LabelSVG(title, width/2, pad+float64(title.Height/2), ` class="pele-title" font-weight="var(--_tw)"`, icons) +
		`<g class="pele-venn-sets">` + sets.String() + `</g>`
	if overlaps.Len() > 0 {
		body += `<g class="pele-venn-overlaps">` + overlaps.String() + `</g>`
	}
	if labels.Len() > 0 {
		body += `<g class="pele-venn-labels">` + labels.String() + `</g>`
	}
	doc := svg.Document("venn", width, height, size, svg.DocumentOptions{
		IDPrefix:       options.IDPrefix,
		Responsive:     rc.Responsive,
		FontFamily:     options.FontFamily,
		FontFamilyMono: options.FontFamilyMono,
	}, svg.Accessible{}, body)
	return core.Rendered{SVG: doc, Width: width, Height: height, Links: []core.Link{}}
}
