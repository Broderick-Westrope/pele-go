package pele_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"testing"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/internal/core"
)

// registeredCorpus returns the corpus and hand-written sources of each registered type.
func registeredCorpus(t testing.TB) map[string][]string {
	t.Helper()
	byType := corpusByType(t)
	for typ, inputs := range handInputs {
		byType[typ] = append(byType[typ], inputs...)
	}
	for typ, sources := range byType {
		if !core.Registered(typ) {
			delete(byType, typ)
			continue
		}
		byType[typ] = portable(sources)
	}
	return byType
}

// TestRenderIsSafeForConcurrentUse renders a sample of every registered type's corpus from
// several goroutines at once, and checks each result against the same render done alone.
func TestRenderIsSafeForConcurrentUse(t *testing.T) {
	const perType, workers, rounds = 6, 8, 3
	byType := registeredCorpus(t)
	var sources []string
	for _, typ := range slices.Sorted(maps.Keys(byType)) {
		sources = append(sources, byType[typ][:min(perType, len(byType[typ]))]...)
	}
	sources = append(sources, "nope")
	opts := pele.Options{Now: fixedNow}
	type outcome struct {
		res pele.Result
		err string
	}
	render := func(src string) outcome {
		res, err := pele.Render(context.Background(), src, opts)
		if err != nil {
			return outcome{err: err.Error()}
		}
		return outcome{res: res}
	}
	want := make([]outcome, len(sources))
	for i, src := range sources {
		want[i] = render(src)
	}
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for r := range rounds {
				for i := range sources {
					k := (i + w*len(sources)/workers + r) % len(sources)
					got := render(sources[k])
					if got.err != want[k].err || got.res.SVG != want[k].res.SVG ||
						!slices.Equal(got.res.Links, want[k].res.Links) {
						t.Errorf("concurrent render of %q differs from a serial one", sources[k])
						return
					}
				}
			}
		})
	}
	wg.Wait()
}

// countingContext is never done but counts how often the render asks.
type countingContext struct {
	context.Context
	calls int
}

func (c *countingContext) Err() error {
	c.calls++
	return nil
}

// TestRenderStopsAtEveryCancellationCheck renders the largest corpus source of each
// registered type with a context that is canceled from its kth check on, for the first
// checks (in Parse, before the renderer) and the last (the deepest the render gets), and
// expects a Canceled error wrapping context.Canceled each time.
func TestRenderStopsAtEveryCancellationCheck(t *testing.T) {
	for typ, sources := range registeredCorpus(t) {
		t.Run(typ, func(t *testing.T) {
			src := ""
			for _, s := range sources {
				if _, err := pele.Render(context.Background(), s, pele.Options{Now: fixedNow}); err == nil && len(s) > len(src) {
					src = s
				}
			}
			if src == "" {
				t.Fatal("no corpus source renders")
			}
			counter := &countingContext{Context: context.Background()}
			if _, err := pele.Render(counter, src, pele.Options{Now: fixedNow}); err != nil {
				t.Fatal(err)
			}
			for _, k := range slices.Compact([]int{1, 2, 3, counter.calls}) {
				if k > counter.calls {
					continue
				}
				ctx := &cancelAfter{Context: context.Background(), n: k}
				_, err := pele.Render(ctx, src, pele.Options{Now: fixedNow})
				var pe *pele.Error
				if !errors.As(err, &pe) || pe.Code != pele.CodeCanceled || !errors.Is(err, context.Canceled) {
					t.Errorf("canceled at check %d of %d: error = %v, want a %s *pele.Error wrapping context.Canceled",
						k, counter.calls, err, pele.CodeCanceled)
				}
				if ctx.calls != k {
					t.Errorf("canceled at check %d: the render checked %d times, want it to stop at the first canceled check", k, ctx.calls)
				}
			}
		})
	}
}

func TestRenderStopsOnAPreCanceledContextForEveryType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for typ, sources := range registeredCorpus(t) {
		t.Run(typ, func(t *testing.T) {
			_, err := pele.Render(ctx, sources[0], pele.Options{Now: fixedNow})
			var pe *pele.Error
			if !errors.As(err, &pe) || pe.Code != pele.CodeCanceled || !errors.Is(err, context.Canceled) {
				t.Fatalf("Render error = %v, want a %s *pele.Error wrapping context.Canceled", err, pele.CodeCanceled)
			}
		})
	}
}
