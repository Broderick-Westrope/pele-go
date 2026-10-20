package pele_test

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestNoFusedMultiplyAdd checks the compiler fused no product into a sum. JS rounds every
// operation; Go may compile a*b+c, even across statements, into one fused multiply-add on
// arm64, which rounds once and can change a coordinate. A product that feeds a sum must be
// rounded on its own with float64(a*b). Calls of math.FMA, which model the fused operations
// of V8's compiled C++, are allowed.
func TestNoFusedMultiplyAdd(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the module")
	}
	t.Parallel()
	cmd := exec.Command("go", "build", "-gcflags=github.com/Broderick-Westrope/pele-go/...=-S", "./...")
	cmd.Env = append(os.Environ(), "GOARCH=arm64", "GOOS=linux", "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	fused := regexp.MustCompile(`\((/[^()]*\.go):(\d+)\)\s+(FN?M(?:ADD|SUB)D)\b`)
	sources := map[string][]string{}
	seen := map[string]bool{}
	for _, m := range fused.FindAllStringSubmatch(string(out), -1) {
		file, line := m[1], m[2]
		if seen[file+":"+line] {
			continue
		}
		seen[file+":"+line] = true
		lines, ok := sources[file]
		if !ok {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			lines = strings.Split(string(data), "\n")
			sources[file] = lines
		}
		n, _ := strconv.Atoi(line)
		if src := lines[n-1]; !strings.Contains(src, "math.FMA(") {
			t.Errorf("%s:%s: %s fused: %s", file, line, m[3], strings.TrimSpace(src))
		}
	}
}
