package layout

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
)

// TestHypotMatchesV8 checks hypot against the Math.hypot results in the fixture. It is an
// internal test because hypot is unexported, and a layout only shows a wrong last bit in it
// when two curves pass at just the distance that decides whether they need levels.
func TestHypotMatchesV8(t *testing.T) {
	f, err := os.Open("testdata/cases.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(z)
	scanner.Buffer(nil, 1<<26)
	parse := func(s string) float64 {
		if s == "-0" {
			return math.Copysign(0, -1)
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	checked, differsFromGo := 0, 0
	for scanner.Scan() {
		var c struct {
			Kind  string      `json:"kind"`
			Input [][3]string `json:"input"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil || c.Kind != "hypot" {
			continue
		}
		for _, in := range c.Input {
			a, b := parse(in[0]), parse(in[1])
			if got := jsnum.Format(hypot(a, b)); got != in[2] {
				t.Errorf("hypot(%s, %s) = %s, want %s", in[0], in[1], got, in[2])
			}
			if jsnum.Format(math.Hypot(a, b)) != in[2] {
				differsFromGo++
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no hypot cases in the fixture")
	}
	t.Logf("%d cases, %d where math.Hypot differs from V8", checked, differsFromGo)
}
