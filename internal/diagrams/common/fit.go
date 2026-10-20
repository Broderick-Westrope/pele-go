package common

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// brTagAt matches <br\s*\/?> at i, ASCII letters in either case, and returns its end or -1.
func brTagAt(units []uint16, i int) int {
	if i+3 > len(units) || units[i] != '<' || units[i+1]|0x20 != 'b' || units[i+2]|0x20 != 'r' {
		return -1
	}
	j := i + 3
	for j < len(units) && jsstr.IsSpace(rune(units[j])) {
		j++
	}
	if j < len(units) && units[j] == '/' {
		j++
	}
	if j < len(units) && units[j] == '>' {
		return j + 1
	}
	return -1
}

// escapeTags is text.replace(/<(?!br\s*\/?>)/gi, '&lt;').
func escapeTags(s string) string {
	units := jsstr.UTF16(s)
	var b strings.Builder
	last := 0
	for i, c := range units {
		if c == '<' && brTagAt(units, i) == -1 {
			b.WriteString(jsstr.FromUTF16(units[last:i]) + "&lt;")
			last = i + 1
		}
	}
	return b.String() + jsstr.FromUTF16(units[last:])
}

// PlainLabel lays out text the way Mermaid's journey and timeline draw it: as plain text in
// which only <br> is markup, so other tags and icon names show as written instead of being
// read as formatting.
func PlainLabel(raw string, measurer text.Measurer, size, max float64, base text.Style) text.Label {
	t := raw
	if strings.Contains(t, "<") {
		t = escapeTags(t)
	}
	if strings.Contains(t, ":fa-") {
		t = strings.ReplaceAll(t, ":fa-", "&colon;fa-")
	}
	// No icon spans can come of the text, so no resolver is needed.
	return text.Layout(t, false, measurer, size, max, base, nil)
}

func isWordUnit(c uint16) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

// unitAt matches /<br\s*\/?>|ﬂ°°?\w+¶ß|&#?\w+;|[\ud800-\udbff][\udc00-\udfff]|[\s\S]/i at i,
// the pieces a word can be cut between, and returns its end.
func unitAt(units []uint16, i int) int {
	if end := brTagAt(units, i); end != -1 {
		return end
	}
	if units[i] == 'ﬂ' && i+1 < len(units) && units[i+1] == '°' {
		j := i + 2
		if j < len(units) && units[j] == '°' {
			j++
		}
		k := j
		for k < len(units) && isWordUnit(units[k]) {
			k++
		}
		if k > j && k+1 < len(units) && units[k] == '¶' && units[k+1] == 'ß' {
			return k + 2
		}
	}
	if units[i] == '&' {
		j := i + 1
		if j < len(units) && units[j] == '#' {
			j++
		}
		k := j
		for k < len(units) && isWordUnit(units[k]) {
			k++
		}
		if k > j && k < len(units) && units[k] == ';' {
			return k + 1
		}
	}
	if units[i] >= 0xd800 && units[i] <= 0xdbff && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] <= 0xdfff {
		return i + 2
	}
	return i + 1
}

// FitLabel lays out text as PlainLabel does, which wraps at spaces only, and for a box of
// fixed width also cuts any word too long to fit.
func FitLabel(raw string, measurer text.Measurer, size, max float64, base text.Style) text.Label {
	label := PlainLabel(raw, measurer, size, max, base)
	if raw == "" || label.Width <= max {
		return label
	}
	units := jsstr.UTF16(raw)
	out := make([]uint16, 0, len(units)+8)
	width := 0.0
	for i := 0; i < len(units); {
		end := unitAt(units, i)
		unit := units[i:end]
		s := jsstr.FromUTF16(unit)
		if jsstr.Trim(s) == "" || len(unit) > 2 && unit[0] == '<' {
			width = 0
		} else {
			measured := s
			if len(unit) > 2 {
				measured = "x"
			}
			w := measurer.Width(measured, size, base)
			if width > 0 && width+w > max {
				out = append(out, '\n')
				width = 0
			}
			width += w
		}
		out = append(out, unit...)
		i = end
	}
	return PlainLabel(jsstr.FromUTF16(out), measurer, size, max, base)
}
