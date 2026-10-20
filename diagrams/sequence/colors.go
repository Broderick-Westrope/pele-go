package sequence

import (
	"regexp"
	"strings"
	"sync"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// Mermaid asks the browser whether the first word of a box title is a color. Pele has no browser,
// so it knows the color keywords and the rgb() and hsl() notations. Other functions, such as calc()
// and var(), are not recognized.
const colorKeywords = "aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue blueviolet brown burlywood " +
	"cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod darkgray " +
	"darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen " +
	"darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey dodgerblue " +
	"firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey honeydew " +
	"hotpink indianred indigo ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan " +
	"lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray " +
	"lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine mediumblue " +
	"mediumorchid mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred " +
	"midnightblue mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid " +
	"palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum powderblue purple " +
	"rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue " +
	"slateblue slategray slategrey snow springgreen steelblue tan teal thistle tomato turquoise violet wheat white " +
	"whitesmoke yellow yellowgreen transparent currentcolor inherit initial unset revert " +
	"accentcolor accentcolortext activetext buttonborder buttonface buttontext canvas canvastext field fieldtext " +
	"graytext highlight highlighttext linktext mark marktext selecteditem selecteditemtext visitedtext " +
	"activeborder activecaption appworkspace background buttonhighlight buttonshadow captiontext inactiveborder " +
	"inactivecaption inactivecaptiontext infobackground infotext menu menutext scrollbar threeddarkshadow threedface " +
	"threedhighlight threedlightshadow threedshadow window windowframe windowtext"

var keywordSet = sync.OnceValue(func() map[string]bool {
	set := map[string]bool{}
	for _, k := range strings.Split(colorKeywords, " ") {
		set[k] = true
	}
	return set
})

// reTerm is upstream's /^[+-]?(?:\d+(?:\.\d+)?|\.\d+)(?:e[+-]?\d+)?(%|deg|grad|rad|turn)?$/i. No
// letter in it has a non-ASCII case variant in either engine.
var reTerm = regexp.MustCompile(`(?i)^[+-]?(?:\d+(?:\.\d+)?|\.\d+)(?:e[+-]?\d+)?(%|deg|grad|rad|turn)?$`)

const (
	termBad = iota
	termNumber
	termPercent
	termAngle
	termNone
)

// lowerASCII is s.toLowerCase() when the result is ASCII, and ok false otherwise. Of the
// non-ASCII characters only the Kelvin sign lowercases to ASCII.
func lowerASCII(s string) (string, bool) {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		r, n := jsstr.Decode(s, i)
		i += n
		switch {
		case r >= 'A' && r <= 'Z':
			b = append(b, byte(r)+32)
		case r < 128:
			b = append(b, byte(r))
		case r == 0x212a:
			b = append(b, 'k')
		default:
			return "", false
		}
	}
	return string(b), true
}

func kind(term string) int {
	if lower, ok := lowerASCII(term); ok && lower == "none" {
		return termNone
	}
	m := reTerm.FindStringSubmatchIndex(term)
	if m == nil {
		return termBad
	}
	if m[2] < 0 {
		return termNumber
	}
	if term[m[2]:m[3]] == "%" {
		return termPercent
	}
	return termAngle
}

func isAlpha(term string, modern bool) bool {
	k := kind(term)
	return k == termNumber || k == termPercent || (modern && k == termNone)
}

// splitSpace is s.split(/\s+/).
func splitSpace(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); {
		r, n := jsstr.Decode(s, i)
		if !jsstr.IsSpace(r) {
			i += n
			continue
		}
		parts = append(parts, s[start:i])
		for i < len(s) {
			r, n := jsstr.Decode(s, i)
			if !jsstr.IsSpace(r) {
				break
			}
			i += n
		}
		start = i
	}
	return append(parts, s[start:])
}

func isFunction(color string) bool {
	hsl := strings.HasPrefix(color, "hsl")
	open := strings.IndexByte(color, '(')
	name := color[:open]
	if name != "rgb" && name != "rgba" && name != "hsl" && name != "hsla" {
		return false
	}
	if jsstr.Len(color) > 120 || strings.IndexByte(color, ')') != len(color)-1 {
		return false
	}
	inner := jsstr.Trim(color[open+1 : len(color)-1])
	var parts []string
	alpha, hasAlpha := "", false
	legacy := strings.Contains(inner, ",")
	if legacy {
		for _, s := range strings.Split(inner, ",") {
			parts = append(parts, jsstr.Trim(s))
		}
		if len(parts) == 4 {
			alpha, hasAlpha = parts[3], true
			parts = parts[:3]
		}
	} else {
		slash := strings.Split(inner, "/")
		if len(slash) > 2 {
			return false
		}
		parts = splitSpace(jsstr.Trim(slash[0]))
		if len(slash) == 2 {
			alpha, hasAlpha = jsstr.Trim(slash[1]), true
		}
	}
	if len(parts) != 3 || (hasAlpha && !isAlpha(alpha, !legacy)) {
		return false
	}
	kinds := [3]int{kind(parts[0]), kind(parts[1]), kind(parts[2])}
	if legacy {
		if hsl {
			return (kinds[0] == termNumber || kinds[0] == termAngle) && kinds[1] == termPercent && kinds[2] == termPercent
		}
		return (kinds[0] == termNumber || kinds[0] == termPercent) && kinds[1] == kinds[0] && kinds[2] == kinds[0]
	}
	for i, k := range kinds {
		var ok bool
		if hsl && i == 0 {
			ok = k == termNumber || k == termNone || k == termAngle
		} else {
			ok = k == termNumber || k == termNone || k == termPercent
		}
		if !ok {
			return false
		}
	}
	return true
}

func isCssColor(color string) bool {
	if strings.Contains(color, "(") {
		return isFunction(color)
	}
	lower, ok := lowerASCII(color)
	return ok && keywordSet()[lower]
}
