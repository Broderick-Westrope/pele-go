package svg

import (
	"strings"
	"unicode"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// RootStyle holds the private aliases set once on the root element, so each use is short
// and the public --pele-* tokens keep their fallbacks.
const RootStyle = "--_bg:var(--pele-bg,#fff);--_fg:var(--pele-fg,#1f1f1f);--_m:var(--pele-muted,#6e6e6e);" +
	"--_l:var(--pele-line,#1f1f1f);--_s:var(--pele-surface,#f3f3f3);--_a:var(--pele-surface-alt,#e6e6e6);" +
	"--_b:var(--pele-border,#8a8a8a);--_c:var(--pele-accent,#5b7bd5);color:var(--_fg)"

// Theme values used across diagram types.
const (
	FontMono = "var(--_fm)"
	Radius   = "var(--pele-radius,4px)"
	Critical = "var(--pele-critical,#e45756)"
)

func isFamilyQuoted(r rune) bool {
	return !strings.ContainsRune("\"'\\\n<>{}();:@", r)
}

func isFamilyStart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-'
}

func isFamilyRest(r rune) bool {
	return isFamilyStart(r) || r == ' ' || r == '.'
}

func skip(s string, i int, ok func(rune) bool) int {
	for i < len(s) {
		r, n := jsstr.Decode(s, i)
		if !ok(r) {
			break
		}
		i += n
	}
	return i
}

// family matches one font name at i: quoted, or a run of letters, digits and _ .- and
// spaces. It returns -1 when there is none.
func family(s string, i int) int {
	if i >= len(s) {
		return -1
	}
	if q := s[i]; q == '"' || q == '\'' {
		end := skip(s, i+1, isFamilyQuoted)
		if end < len(s) && s[end] == q {
			return end + 1
		}
		return -1
	}
	if r, n := jsstr.Decode(s, i); isFamilyStart(r) {
		return skip(s, i+n, isFamilyRest)
	}
	return -1
}

// isFamilies reports whether list is a list of font names, quoted or not, and nothing that
// could end the declaration it is put in:
// /^\s*FAMILY(?:\s*,\s*FAMILY)*\s*$/u
func isFamilies(list string) bool {
	i := family(list, skip(list, 0, jsstr.IsSpace))
	if i == -1 {
		return false
	}
	for {
		j := skip(list, i, jsstr.IsSpace)
		if j < len(list) && list[j] == ',' {
			if next := family(list, skip(list, j+1, jsstr.IsSpace)); next != -1 {
				i = next
				continue
			}
		}
		return j == len(list)
	}
}

func families(list *string, generic string) string {
	if list != nil && *list != "" && isFamilies(*list) {
		return *list
	}
	return generic
}

// FontStyle returns the root's font declarations. The fonts the labels were measured in are
// the ones they are drawn in, unless the page names others.
func FontStyle(size float64, family, mono *string) string {
	return ";--_fm:var(--pele-font-mono," + families(mono, "monospace") + ");font:" + jsnum.Format(size) + "px sans-serif;" +
		"letter-spacing:normal;word-spacing:normal;text-transform:none;direction:ltr;" +
		"font-family:var(--pele-font," + families(family, "sans-serif") + ")"
}
