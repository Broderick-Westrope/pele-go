package util_test

import (
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/fixture"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

func options(o *jsobj.Object) core.Options {
	var opts core.Options
	for _, key := range o.Keys() {
		v, _ := o.Get(key)
		switch key {
		case "links", "images":
			b := v.(bool)
			if key == "links" {
				opts.Links = &b
			} else {
				opts.Images = &b
			}
		case "linkRel":
			s := v.(string)
			opts.LinkRel = &s
		case "linkSchemes", "imageSchemes":
			list := []string{}
			for _, e := range v.([]any) {
				list = append(list, e.(string))
			}
			if key == "linkSchemes" {
				opts.LinkSchemes = list
			} else {
				opts.ImageSchemes = list
			}
		}
	}
	return opts
}

func orNil(s string, ok bool) any {
	if !ok {
		return nil
	}
	return s
}

// TestCases replays upstream's results from testdata/cases.json (scripts/gen-shared-cases.mjs).
func TestCases(t *testing.T) {
	fixture.Run(t, "testdata/cases.json", map[string]fixture.Func{
		"letters": func(fixture.Args) any {
			ranges := []any{}
			for c := 0; c <= 0x10000; c++ {
				if !util.IsUnicodeLetter(c) {
					continue
				}
				if n := len(ranges); n > 0 {
					if last := ranges[n-1].([]any); last[1].(float64) == float64(c-1) {
						last[1] = float64(c)
						continue
					}
				}
				ranges = append(ranges, []any{float64(c), float64(c)})
			}
			return ranges
		},
		"sanitizeUrl": func(a fixture.Args) any { return util.SanitizeURL(a.Str(0)) },
		"safeUrl":     func(a fixture.Args) any { return util.SafeURL(a.Str(0)) },
		"linkUrl":     func(a fixture.Args) any { return orNil(util.LinkURL(a.Str(0), options(a.Obj(1)))) },
		"imageUrl":    func(a fixture.Args) any { return orNil(util.ImageURL(a.Str(0), options(a.Obj(1)))) },
		"linked": func(a fixture.Args) any {
			var links []core.Link
			out := util.Linked("<b/>", a.Str(0), `id"1`, &links, options(a.Obj(1)), ` target="_blank"`)
			list := []any{}
			for _, l := range links {
				list = append(list, map[string]any{"id": l.ID, "href": l.Href, "internal": l.Internal})
			}
			return map[string]any{"svg": out, "links": list}
		},
		"relAttr": func(a fixture.Args) any { return util.RelAttr(options(a.Obj(0))) },
		"chars": func(fixture.Args) any {
			tests := []func(int) bool{util.IsSpace, util.IsBlank, util.IsLineEnd, util.IsDigit, util.IsLetter, util.IsHex, util.IsWord}
			mask := func(c int) float64 {
				m := 0
				for k, f := range tests {
					if f(c) {
						m |= 1 << k
					}
				}
				return float64(m)
			}
			out := []any{}
			for c := 0; c <= 0x10000; c++ {
				if m := mask(c); m != 0 {
					out = append(out, []any{float64(c), m})
				}
			}
			return append(out, []any{-1.0, mask(-1)})
		},
	})
}
