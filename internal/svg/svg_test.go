package svg_test

import (
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/fixture"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/svg"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

func num(o *jsobj.Object, key string) float64 {
	v, _ := o.Get(key)
	f, _ := v.(float64)
	return f
}

func box(o *jsobj.Object) svg.Box {
	group, _ := o.Get("isGroup")
	return svg.Box{X: num(o, "x"), Y: num(o, "y"), W: num(o, "w"), H: num(o, "h"), IsGroup: group == true}
}

func label(s string) text.Label {
	return text.Layout(s, false, text.Metrics, 14, 200, 0, nil)
}

func edgePath(p svg.EdgePath) map[string]any {
	return map[string]any{"d": p.D, "sx": p.SX, "sy": p.SY, "sdx": p.SDX, "sdy": p.SDY, "ex": p.EX, "ey": p.EY, "edx": p.EDX, "edy": p.EDY}
}

func orNil[T comparable](v T) any {
	var zero T
	if v == zero {
		return nil
	}
	return v
}

// TestCases replays upstream's results from testdata/cases.json (scripts/gen-shared-cases.mjs).
func TestCases(t *testing.T) {
	fixture.Run(t, "testdata/cases.json", map[string]fixture.Func{
		"resolveStyle": func(a fixture.Args) any {
			r := svg.ResolveStyle(a.Strs(0))
			return map[string]any{
				"shape": r.Shape, "line": r.Line, "text": r.Text, "stroke": orNil(r.Stroke),
				"bold": r.Bold, "italic": r.Italic, "fontSize": orNil(r.FontSize),
			}
		},
		"classNames":  func(a fixture.Args) any { return svg.ClassNames(a.Str(0)) },
		"seriesColor": func(a fixture.Args) any { return svg.SeriesColor(int(a.Num(0))) },
		"withTitle": func(a fixture.Args) any {
			r := svg.WithTitle(a.Str(0), text.Metrics, a.Num(1), a.Num(2), a.Num(3), a.Num(4), a.Str(5), nil)
			return map[string]any{"width": r.Width, "height": r.Height, "content": r.Content}
		},
		"cluster": func(a fixture.Args) any {
			c := box(a.Obj(0))
			crossings := svg.Crossings{}
			for _, route := range a[2].([]any) {
				svg.MarkCrossings(crossings, fixture.Args{route}.Nums(0))
			}
			x := svg.ClusterTitleX(c, a.Num(1), crossings, a.Num(3))
			return []any{x, svg.TitleCrossed(c, a.Num(1), x, crossings), svg.TitleCrossed(c, a.Num(1), a.Num(4), crossings)}
		},
		"struckTitle": func(a fixture.Args) any {
			parents := a.Nums(5)
			parent := func(i int) int { return int(parents[i]) }
			return svg.StruckTitle(label(a.Str(0)), a.Num(1), a.Num(2), a.Str(3), a.Str(4), parent, int(a.Num(6)), nil)
		},
		"routePath": func(a fixture.Args) any {
			startReach, endReach := svg.DefaultReach(a.Num(2)), svg.DefaultReach(a.Num(3))
			if !a.Undefined(4) {
				startReach, endReach = a.Num(4), a.Num(5)
			}
			curve := ""
			if !a.Undefined(1) {
				curve = a.Str(1)
			}
			return edgePath(svg.RoutePath(a.Nums(0), curve, a.Num(2), a.Num(3), startReach, endReach))
		},
		"markerTrim": func(a fixture.Args) any { return svg.MarkerTrim(a.Str(0)) },
		"marker": func(a fixture.Args) any {
			return svg.Marker(a.Str(0), a.Num(1), a.Num(2), a.Num(3), a.Num(4), a.Str(5))
		},
		"edgeLabelSvg": func(a fixture.Args) any {
			return svg.EdgeLabelSVG(a.Str(0), label(a.Str(1)), a.Num(2), a.Num(3), a.Str(4), nil)
		},
		"loopPath": func(a fixture.Args) any {
			p, x, y := svg.LoopPath(a.Num(0), a.Num(1), a.Num(2), a.Num(3), a.Num(4), a.Bool(5), a.Num(6), a.Num(7), a.Num(8), a.Num(9))
			return map[string]any{"path": edgePath(p), "labelX": x, "labelY": y}
		},
		"sideReach": func(a fixture.Args) any {
			var reach svg.SideReach
			for _, op := range a[0].([]any) {
				op := op.([]any)
				reach.Add(int(op[0].(float64)), int(op[1].(float64)), op[2].(string))
			}
			var out []any
			for _, q := range a[1].([]any) {
				q := q.([]any)
				out = append(out, reach.Of(int(q[0].(float64)), int(q[1].(float64))))
			}
			return out
		},
		"shapeSize": func(a fixture.Args) any {
			s := svg.ShapeSizeOf(a.Str(0), a.Num(1), a.Num(2))
			return map[string]any{"w": s.W, "h": s.H, "dy": s.DY}
		},
		"drawShape":     func(a fixture.Args) any { return svg.DrawShape(a.Str(0), a.Num(1), a.Num(2), a.Str(3), a.Str(4)) },
		"shapeInset":    func(a fixture.Args) any { return svg.ShapeInset(a.Str(0), a.Num(1), a.Num(2), int(a.Num(3)), a.Num(4)) },
		"shapeSpan":     func(a fixture.Args) any { return svg.ShapeSpan(a.Str(0), a.Num(1), a.Num(2), a.Bool(3)) },
		"shapeHasLabel": func(a fixture.Args) any { return svg.ShapeHasLabel(a.Str(0)) },
		"insetRoute": func(a fixture.Args) any {
			route := a.Nums(0)
			c := box(a.Obj(1))
			view := a.Obj(2)
			shape, _ := view.Get("shape")
			side := svg.EndSide(route, 0, c)
			svg.InsetRoute(route, 0, shape.(string), num(view, "w"), num(view, "h"), c)
			return []any{float64(side), route}
		},
	})
}
