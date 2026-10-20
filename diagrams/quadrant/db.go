package quadrant

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type quadrantText struct {
	text     string
	markdown bool
}

// pointStyle holds a point's or a class's styles; nil fields are unset.
type pointStyle struct {
	radius      *float64
	color       *string
	strokeColor *string
	strokeWidth *string
}

type quadrantPoint struct {
	pointStyle
	x, y      float64
	text      quadrantText
	className string
}

type quadrantDb struct {
	title, accTitle, accDescr *string
	// quadrants 1 to 4: top right, top left, bottom left, bottom right.
	quadrants                                    [4]*quadrantText
	xAxisLeft, xAxisRight, yAxisBottom, yAxisTop *quadrantText
	points                                       []quadrantPoint
	classes                                      map[string]pointStyle
}

func newQuadrantDb() *quadrantDb {
	return &quadrantDb{classes: map[string]pointStyle{}}
}

func invalid(style, value, kind string) *perr.Error {
	e := perr.New("value for "+style+" "+value+" is invalid, please use a valid "+kind, perr.Semantic)
	e.Type = "quadrantChart"
	return e
}

// keyValue returns the first two parts of Mermaid's style.trim().split(/\s*:\s*/), found in
// one pass.
func keyValue(style string) (string, string) {
	s := jsstr.Trim(style)
	colon := strings.IndexByte(s, ':')
	if colon == -1 {
		return s, "undefined"
	}
	rest := jsstr.TrimStart(s[colon+1:])
	next := strings.IndexByte(rest, ':')
	if next == -1 {
		return jsstr.TrimEnd(s[:colon]), rest
	}
	return jsstr.TrimEnd(s[:colon]), jsstr.TrimEnd(rest[:next])
}

// isNumber is /^\d+$/.
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isHex is /^#?([\dA-Fa-f]{6}|[\dA-Fa-f]{3})$/.
func isHex(s string) bool {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 && len(s) != 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// isPixels is /^\d+px$/.
func isPixels(s string) bool {
	digits, ok := strings.CutSuffix(s, "px")
	return ok && isNumber(digits)
}

func parseStyles(styles []string) pointStyle {
	var out pointStyle
	for _, style := range styles {
		key, value := keyValue(style)
		switch key {
		case "radius":
			if !isNumber(value) {
				perr.Throw(invalid(key, value, "number"))
			}
			r := jsnum.ParseInt(value)
			out.radius = &r
		case "color":
			if !isHex(value) {
				perr.Throw(invalid(key, value, "hex code"))
			}
			out.color = &value
		case "stroke-color":
			if !isHex(value) {
				perr.Throw(invalid(key, value, "hex code"))
			}
			out.strokeColor = &value
		case "stroke-width":
			if !isPixels(value) {
				perr.Throw(invalid(key, value, "number of pixels (eg. 10px)"))
			}
			out.strokeWidth = &value
		default:
			e := perr.New("style named "+key+" is not supported.", perr.Semantic)
			e.Type = "quadrantChart"
			perr.Throw(e)
		}
	}
	return out
}

func trimmed(label quadrantText) *quadrantText {
	return &quadrantText{text: jsstr.Trim(label.text), markdown: label.markdown}
}

// setQuadrantText stands for upstream's setQuadrant1Text to setQuadrant4Text.
func (db *quadrantDb) setQuadrantText(k int, label quadrantText) { db.quadrants[k] = trimmed(label) }

func (db *quadrantDb) setXAxisLeftText(label quadrantText) { db.xAxisLeft = trimmed(label) }

func (db *quadrantDb) setXAxisRightText(label quadrantText) { db.xAxisRight = trimmed(label) }

func (db *quadrantDb) setYAxisBottomText(label quadrantText) { db.yAxisBottom = trimmed(label) }

func (db *quadrantDb) setYAxisTopText(label quadrantText) { db.yAxisTop = trimmed(label) }

func (db *quadrantDb) addPoint(label quadrantText, className, x, y string, styles []string) {
	db.points = append(db.points, quadrantPoint{
		x:          jsnum.Parse(x),
		y:          jsnum.Parse(y),
		text:       *trimmed(label),
		className:  className,
		pointStyle: parseStyles(styles),
	})
}

func (db *quadrantDb) addClass(className string, styles []string) {
	db.classes[className] = parseStyles(styles)
}

func (db *quadrantDb) setDiagramTitle(text string) { db.title = &text }

func (db *quadrantDb) setAccTitle(text string) {
	s := jsstr.TrimStart(text)
	db.accTitle = &s
}

func (db *quadrantDb) setAccDescription(text string) {
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
