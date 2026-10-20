package jsmath_test

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
)

// TestPowMatchesV8 replays x ** y as Node computed it (testdata/pow.json, from
// scripts/gen-pow-cases.mjs), bit for bit.
func TestPowMatchesV8(t *testing.T) {
	data, err := os.ReadFile("testdata/pow.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases [][3]string
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	double := func(s string) float64 {
		b, err := strconv.ParseUint(s, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		return math.Float64frombits(b)
	}
	failures := 0
	for _, c := range cases {
		x, y, want := double(c[0]), double(c[1]), double(c[2])
		got := jsmath.Pow(x, y)
		same := math.Float64bits(got) == math.Float64bits(want) || (math.IsNaN(got) && math.IsNaN(want))
		if !same {
			t.Errorf("Pow(%v, %v) = %v, want %v", x, y, got, want)
			if failures++; failures > 20 {
				t.Fatal("too many failures")
			}
		}
	}
}
