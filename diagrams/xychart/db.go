package xychart

import (
	"math"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

type dataPoint struct {
	value float64
	label string
}

// axis is upstream's BandAxis or LinearAxis.
type axis struct {
	band       bool
	title      string
	categories []string
	min, max   float64
}

// plotValue is a value with its place on the x axis: a category, or a number written as
// text. A category with no value has has false.
type plotValue struct {
	at    string
	value float64
	has   bool
}

type plot struct {
	bar   bool
	title string
	data  []plotValue
	// pointLabels is nil where upstream's is undefined.
	pointLabels []string
}

type xyChartDb struct {
	title, accTitle, accDescr *string
	orientation               string
	xAxis                     axis
	yAxis                     axis
	hasSetXAxis               bool
	// hasSetYAxis is false while the y range comes from the data and not from a y-axis
	// statement.
	hasSetYAxis bool
	plots       []plot
}

func newXyChartDb() *xyChartDb {
	return &xyChartDb{
		xAxis: axis{band: true},
		yAxis: axis{min: math.Inf(1), max: math.Inf(-1)},
	}
}

func (db *xyChartDb) setOrientation(orientation string) {
	if orientation == "horizontal" {
		db.orientation = "horizontal"
	} else {
		db.orientation = "vertical"
	}
}

func (db *xyChartDb) setXAxisTitle(title string) { db.xAxis.title = jsstr.Trim(title) }

func (db *xyChartDb) setXAxisRangeData(lo, hi float64) {
	db.xAxis = axis{title: db.xAxis.title, min: lo, max: hi}
	db.hasSetXAxis = true
}

func (db *xyChartDb) setXAxisBand(categories []string) {
	trimmed := make([]string, len(categories))
	for i, c := range categories {
		trimmed[i] = jsstr.Trim(c)
	}
	db.xAxis = axis{band: true, title: db.xAxis.title, categories: trimmed}
	db.hasSetXAxis = true
}

func (db *xyChartDb) setYAxisTitle(title string) { db.yAxis.title = jsstr.Trim(title) }

func (db *xyChartDb) setYAxisRangeData(lo, hi float64) {
	db.yAxis = axis{title: db.yAxis.title, min: lo, max: hi}
	db.hasSetYAxis = true
}

func (db *xyChartDb) plotData(values []float64) []plotValue {
	if len(values) == 0 {
		return nil
	}
	// The first series fixes the x range when no x-axis statement did.
	if !db.hasSetXAxis {
		db.setXAxisRangeData(1, float64(len(values)))
	}
	x := db.xAxis
	if x.band && len(values) > len(x.categories) {
		values = values[:len(x.categories)]
	}
	if !db.hasSetYAxis {
		lo, hi := db.yAxis.min, db.yAxis.max
		for _, value := range values {
			if value < lo {
				lo = value
			}
			if value > hi {
				hi = value
			}
		}
		db.yAxis = axis{title: db.yAxis.title, min: lo, max: hi}
	}
	if x.band {
		out := make([]plotValue, len(x.categories))
		for i, category := range x.categories {
			out[i] = plotValue{at: category}
			if i < len(values) {
				out[i].value, out[i].has = values[i], true
			}
		}
		return out
	}
	if len(values) == 1 {
		return []plotValue{{at: jsnum.Format(x.min), value: values[0], has: true}}
	}
	step := (x.max - x.min) / float64(len(values)-1)
	out := make([]plotValue, len(values))
	for i, value := range values {
		out[i] = plotValue{at: jsnum.Format(x.min + float64(float64(i)*step)), value: value, has: true}
	}
	return out
}

func (db *xyChartDb) addPlot(bar bool, title string, data []dataPoint) {
	values := make([]float64, len(data))
	for i, d := range data {
		values[i] = d.value
	}
	p := plot{bar: bar, title: jsstr.Trim(title), data: db.plotData(values)}
	// Mermaid accepts labels on the points of a bar series and ignores them.
	if !bar {
		labelled := false
		for _, d := range data {
			if jsstr.Trim(d.label) != "" {
				labelled = true
			}
		}
		if labelled {
			p.pointLabels = make([]string, len(data))
			for i, d := range data {
				p.pointLabels[i] = jsstr.Trim(d.label)
			}
		}
	}
	db.plots = append(db.plots, p)
}

func (db *xyChartDb) setLineData(title string, data []dataPoint) { db.addPlot(false, title, data) }

func (db *xyChartDb) setBarData(title string, data []dataPoint) { db.addPlot(true, title, data) }

func (db *xyChartDb) setDiagramTitle(text string) { db.title = &text }

func (db *xyChartDb) setAccTitle(text string) {
	s := jsstr.TrimStart(text)
	db.accTitle = &s
}

func (db *xyChartDb) setAccDescription(text string) {
	s := collapseIndentedBreaks(text)
	db.accDescr = &s
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
