package svg_test

import (
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/svg"
)

// Upstream's tests/render/edges.test.ts, where it asserts whole paths.
func TestRoutePath(t *testing.T) {
	corner := []float64{0, 0, 0, 0, 20, 0, 100, 20, 0, 100, 48, 0}
	tests := map[string]struct {
		route              []float64
		curve              string
		startTrim, endTrim float64
		want               string
	}{
		"straight into an arrowhead": {route: []float64{0, 0, 0, 0, 48, 0}, endTrim: 7, want: "M0,0L0,41"},
		"stepped corner":             {route: corner, curve: "step", want: "M0,0L0,8Q0,20 12,20L88,20Q100,20 100,32L100,48"},
		"short middle run":           {route: []float64{0, 0, 0, 0, 20, 0, 10, 20, 0, 10, 48, 0}, curve: "step", want: "M0,0L0,15Q0,20 5,20Q10,20 10,25L10,48"},
		"linear corner":              {route: corner, curve: "linear", want: "M0,0L0,20L100,20L100,48"},
		"point along a straight run": {route: []float64{0, 0, 0, 0, 20, 0, 0, 48, 0}, want: "M0,0L0,48"},
		"step with an arrowhead":     {route: []float64{0, 0, 0, 100, 48, 0}, curve: "step", endTrim: 7, want: "M0,0L0,12Q0,24 12,24L88,24Q100,24 100,36L100,41"},
		"step after":                 {route: []float64{0, 0, 0, 100, 48, 0}, curve: "stepAfter", want: "M0,0L0,36Q0,48 12,48L100,48"},
		"step before":                {route: []float64{0, 0, 1, 48, 100, 1}, curve: "stepBefore", want: "M0,0L0,88Q0,100 12,100L48,100"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := svg.RoutePath(tc.route, tc.curve, tc.startTrim, tc.endTrim, svg.DefaultReach(tc.startTrim), svg.DefaultReach(tc.endTrim))
			if got.D != tc.want {
				t.Errorf("d = %s, want %s", got.D, tc.want)
			}
		})
	}
	leaving := svg.RoutePath([]float64{0, 0, 0, 40, 48, 0}, "", 7, 0, svg.DefaultReach(7), 0)
	if !strings.HasPrefix(leaving.D, "M0,7L0,13C") {
		t.Errorf("leaving d = %s, want it to run straight out of the arrowhead", leaving.D)
	}
	arrow := svg.RoutePath([]float64{0, 0, 0, 40, 48, 0}, "", 0, 7, 0, svg.DefaultReach(7))
	if !strings.HasSuffix(arrow.D, " 40,35L40,41") {
		t.Errorf("arriving d = %s, want it to run straight into the arrowhead", arrow.D)
	}
}
