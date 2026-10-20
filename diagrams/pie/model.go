package pie

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type model struct {
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
	showData                  bool
	sections                  *omap.Map[string, float64]
}

func buildPie(a ast, title *string) model {
	sections := &omap.Map[string, float64]{}
	for _, s := range a.sections {
		if s.value < 0 {
			e := perr.New(`"`+s.label+`" has invalid value: `+jsnum.Format(s.value)+
				`. Negative values are not allowed in pie charts. All slice values must be >= 0.`, perr.Semantic)
			e.Type = "pie"
			perr.Throw(e)
		}
		if !sections.Has(s.label) {
			sections.Set(s.label, s.value)
		}
	}
	m := model{showData: a.showData, sections: sections}
	if a.title != nil && *a.title != "" {
		m.title = *a.title
	} else if title != nil {
		m.title = *title
	}
	if a.accTitle != nil && *a.accTitle != "" {
		m.accTitle = jsstr.TrimStart(*a.accTitle)
	}
	if a.accDescr != nil && *a.accDescr != "" {
		m.accDescr = collapseIndentedBreaks(*a.accDescr)
	}
	return m
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
		b.WriteByte('\n')
		i++
		for i < len(s) {
			r, n := jsstr.Decode(s, i)
			if !jsstr.IsSpace(r) {
				break
			}
			i += n
		}
	}
	return b.String()
}
