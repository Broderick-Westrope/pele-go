package cynefin

import (
	"math"
	"testing"
)

// The values are V8's, from upstream's boundaries.ts under Node.
func TestSeededRandom(t *testing.T) {
	tests := map[string]struct {
		seed, want float64
	}{
		"zero":       {0, 0.26642920868471265},
		"one":        {1, 0.6270739405881613},
		"42":         {42, 0.6011037519201636},
		"negative":   {-7, 0.43306733411736786},
		"fraction":   {1.5, 0.6270739405881613},
		"large":      {1e15, 0.4975225117523223},
		"large neg":  {-1e15, 0.30435564112849534},
		"big + frac": {123456789.75, 0.2577907438389957},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := seededRandom(tc.seed); got != tc.want {
				t.Errorf("seededRandom(%v) = %v, want %v", tc.seed, got, tc.want)
			}
		})
	}
}

func TestHashString(t *testing.T) {
	tests := map[string]float64{"pele": 3437102, "": 0, "a long id": 1054607488, "é😀": 1996812}
	for s, want := range tests {
		if got := hashString(s); got != want {
			t.Errorf("hashString(%q) = %v, want %v", s, got, want)
		}
	}
}

// Ported from tests/compat/cynefin/cynefin.spec.ts.
func TestResolveSeed(t *testing.T) {
	tests := map[string]struct {
		configured float64
		id         string
		want       float64
	}{
		"configured":       {5, "x", 5},
		"zero falls back":  {0, "x", 120},
		"NaN falls back":   {math.NaN(), "pele", 3437102},
		"infinity as none": {math.Inf(1), "pele", 3437102},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := resolveSeed(tc.configured, tc.id); got != tc.want {
				t.Errorf("resolveSeed(%v, %q) = %v, want %v", tc.configured, tc.id, got, tc.want)
			}
		})
	}
}

// Ported from tests/compat/cynefin/cynefin.spec.ts: self-loops are dropped and an empty label
// is none.
func TestSetTransitions(t *testing.T) {
	db := &cynefinDb{domains: map[string][]string{}}
	db.setTransitions([]transitionAst{{from: "complex", to: "complex"}, {from: "clear", to: "chaotic", label: new("")}, {from: "complex", to: "clear", label: new("x")}})
	want := []cynefinTransition{{"clear", "chaotic", ""}, {"complex", "clear", "x"}}
	if len(db.transitions) != len(want) || db.transitions[0] != want[0] || db.transitions[1] != want[1] {
		t.Errorf("transitions = %+v, want %+v", db.transitions, want)
	}
}
