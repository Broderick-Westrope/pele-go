package jsnum_test

import (
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/fixture"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
)

// TestFixedMatchesV8 replays toFixed, toExponential and toPrecision as Node computed them
// (testdata/fixed.json, from scripts/gen-jsnum-fixed-cases.mjs).
func TestFixedMatchesV8(t *testing.T) {
	fixture.Run(t, "testdata/fixed.json", map[string]fixture.Func{
		"toFixed":       func(a fixture.Args) any { return jsnum.ToFixed(jsnum.Parse(a.Str(0)), int(a.Num(1))) },
		"toExponential": func(a fixture.Args) any { return jsnum.ToExponential(jsnum.Parse(a.Str(0)), int(a.Num(1))) },
		"toPrecision":   func(a fixture.Args) any { return jsnum.ToPrecision(jsnum.Parse(a.Str(0)), int(a.Num(1))) },
	})
}
