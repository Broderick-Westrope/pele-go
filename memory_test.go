package pele_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/fontmeasure"
	"github.com/Broderick-Westrope/pele-go/internal/oracle"
)

func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

func dejaVuMeasurer(t testing.TB) pele.TextMeasurer {
	t.Helper()
	var faces [4][]byte
	for i, name := range []string{"DejaVuSans.ttf", "DejaVuSans-Bold.ttf", "DejaVuSans-Oblique.ttf", "DejaVuSans-BoldOblique.ttf"} {
		data, err := os.ReadFile(filepath.Join("fontmeasure", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		faces[i] = data
	}
	m, err := fontmeasure.New(faces[0], faces[1], faces[2], faces[3])
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestRenderRetainsLittleMemory checks the design's library budget: rendering every corpus
// diagram with the fontmeasure measurer leaves the heap in use, after GC, within 2 MB of
// where it started. A first pass builds the tables made once per process, which the budget
// does not count. It logs the peak, and what loading the goja oracle costs for comparison.
func TestRenderRetainsLittleMemory(t *testing.T) {
	var sources []string
	for _, srcs := range registeredCorpus(t) {
		sources = append(sources, srcs...)
	}
	opts := pele.Options{Measurer: dejaVuMeasurer(t), Now: fixedNow}
	renderAll := func() (peak uint64) {
		for _, src := range sources {
			_, _ = pele.Render(context.Background(), src, opts)
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			peak = max(peak, m.HeapInuse)
		}
		return peak
	}
	renderAll()
	before := heapInUse()
	var peak uint64
	for range 3 {
		peak = max(peak, renderAll())
	}
	after := heapInUse()
	retained := int64(after) - int64(before)
	t.Logf("Go port, %d sources: retained %d KB, peak heap in use %d KB above start",
		len(sources), retained/1024, (int64(peak)-int64(before))/1024)
	if retained > 2<<20 {
		t.Errorf("retained %d bytes after rendering, want at most 2 MB", retained)
	}

	infoSources := corpus(t, "info-docs.json")
	beforeOracle := heapInUse()
	o := newOracle(t)
	for _, src := range infoSources {
		_, _, _ = o.Render(src, oracleOptions())
	}
	t.Logf("goja oracle with Pele loaded: %d KB held", (int64(heapInUse())-int64(beforeOracle))/1024)
	runtime.KeepAlive(o)
}

func oracleOptions() oracle.Options { return oracle.Options{Now: fixedNow} }
