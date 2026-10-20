package jsmath_test

import (
	"math"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/fixture"
	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
)

// TestAcosMatchesV8 replays Math.acos as Node computed it (testdata/acos.json, from
// scripts/gen-acos-cases.mjs).
func TestAcosMatchesV8(t *testing.T) {
	fixture.Run(t, "testdata/acos.json", map[string]fixture.Func{
		"acos": func(a fixture.Args) any { return jsmath.Acos(a.Num(0)) },
	})
}

func TestAcosNonFinite(t *testing.T) {
	for _, x := range []float64{math.Inf(1), math.Inf(-1), math.NaN()} {
		if got := jsmath.Acos(x); !math.IsNaN(got) {
			t.Errorf("Acos(%v) = %v, want NaN", x, got)
		}
	}
}
