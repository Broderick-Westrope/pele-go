package jsmath_test

import (
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/fixture"
	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
)

// TestMatchesV8 replays Math.sin, Math.cos and Math.hypot as Node computed them
// (testdata/cases.json, from scripts/gen-shared-cases.mjs).
func TestMatchesV8(t *testing.T) {
	fixture.Run(t, "testdata/cases.json", map[string]fixture.Func{
		"sin":   func(a fixture.Args) any { return jsmath.Sin(a.Num(0)) },
		"cos":   func(a fixture.Args) any { return jsmath.Cos(a.Num(0)) },
		"hypot": func(a fixture.Args) any { return jsmath.Hypot(a.AllNums()...) },
	})
}
