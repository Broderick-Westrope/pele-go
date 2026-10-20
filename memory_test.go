package pele_test

import (
	"context"
	"runtime"
	"testing"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/internal/oracle"
)

func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

// TestRenderRetainsLittleMemory checks the design's library budget: after rendering, the
// heap the renderer keeps is within 2 MB of where it started. It logs what loading the
// goja oracle costs for comparison.
func TestRenderRetainsLittleMemory(t *testing.T) {
	sources := append(corpus(t, "info-docs.json"), infoInputs...)
	before := heapInUse()
	var peak uint64
	for range 20 {
		for _, src := range sources {
			_, _ = pele.Render(context.Background(), src, pele.Options{})
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		peak = max(peak, m.HeapInuse)
	}
	after := heapInUse()
	retained := int64(after) - int64(before)
	t.Logf("Go port: retained %d KB, peak heap in use %d KB above start", retained/1024, (int64(peak)-int64(before))/1024)
	if retained > 2<<20 {
		t.Errorf("retained %d bytes after rendering, want at most 2 MB", retained)
	}

	beforeOracle := heapInUse()
	o := newOracle(t)
	for _, src := range sources {
		_, _, _ = o.Render(src, oracleOptions())
	}
	t.Logf("goja oracle with Pele loaded: %d KB held", (int64(heapInUse())-int64(beforeOracle))/1024)
	runtime.KeepAlive(o)
}

func oracleOptions() oracle.Options { return oracle.Options{Now: fixedNow} }
