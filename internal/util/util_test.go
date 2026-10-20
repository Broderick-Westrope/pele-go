package util_test

import (
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/fixture"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

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
