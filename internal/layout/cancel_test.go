package layout_test

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Broderick-Westrope/pele-go/internal/layout"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

// countdown is a context that reports itself canceled once Err has been asked for `left` times,
// which stops a layout part-way through.
type countdown struct {
	left atomic.Int64
}

func (c *countdown) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *countdown) Done() <-chan struct{}       { return nil }
func (c *countdown) Value(any) any               { return nil }
func (c *countdown) Err() error {
	if c.left.Add(-1) < 0 {
		return context.Canceled
	}
	return nil
}

func canceledError(t *testing.T, run func()) (err *perr.Error) {
	t.Helper()
	defer func() {
		r := recover()
		e, ok := r.(*perr.Error)
		if !ok {
			t.Fatalf("recovered %v, want a *perr.Error", r)
		}
		err = e
	}()
	run()
	return nil
}

func TestLayoutsStopWhenTheContextIsDone(t *testing.T) {
	lanes := randomLanes(7)
	nodes, edges, dir := randomCompound(9)
	runs := map[string]func(ctx context.Context){
		"layered": func(ctx context.Context) {
			ns := []layout.LNode{layout.NewLNode(10, 10, layout.KindNode), layout.NewLNode(10, 10, layout.KindNode)}
			layout.Layered(ctx, ns, []layout.LEdge{layout.NewLEdge(0, 1, 1, 0, 0)}, testOptions)
		},
		"compound": func(ctx context.Context) {
			layout.CompoundLayout(ctx, append([]layout.CNode{}, nodes...), append([]layout.CEdge{}, edges...), dir, testOptions)
		},
		"lanes": func(ctx context.Context) {
			layout.LaneLayout(ctx, append([]layout.CNode{}, lanes.nodes...), append([]layout.CEdge{}, lanes.edges...), lanes.dir, testOptions, layout.LanePad)
		},
	}
	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := canceledError(t, func() { run(ctx) })
			if err.Code != perr.Canceled || !errors.Is(err, context.Canceled) {
				t.Errorf("got %v (code %s), want a Canceled error wrapping context.Canceled", err, err.Code)
			}

			// Count the checks one whole layout makes, then stop it at each of them in turn.
			counter := &countdown{}
			counter.left.Store(1 << 40)
			run(counter)
			checks := int(1<<40 - counter.left.Load())
			if checks < 2 {
				t.Fatalf("a layout checks its context %d times", checks)
			}
			for k := 1; k < checks; k++ {
				c := &countdown{}
				c.left.Store(int64(k))
				if err := canceledError(t, func() { run(c) }); err.Code != perr.Canceled {
					t.Fatalf("stopped after %d checks: code %s", k, err.Code)
				}
			}
		})
	}
}

// TestNoFusedMultiplyAdd compiles the package for targets with fused multiply-add and fails if
// the compiler fused any product into a sum, which rounds once where V8 rounds twice.
func TestNoFusedMultiplyAdd(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles the package for other targets")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool on PATH")
	}
	fused := regexp.MustCompile(`\b(V?F(N)?M(ADD|SUB)[A-Z0-9]*)\b`)
	line := regexp.MustCompile(`\(([^()]*\.go:\d+)\)`)
	for _, target := range []struct{ goarch, goamd64 string }{{"arm64", ""}, {"amd64", "v3"}, {"ppc64le", ""}, {"s390x", ""}, {"riscv64", ""}} {
		cmd := exec.Command(goTool, "build", "-gcflags=-S", "-o", filepath.Join(t.TempDir(), "out"), ".")
		cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH="+target.goarch, "GOAMD64="+target.goamd64, "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", target.goarch, err, out)
		}
		for _, l := range strings.Split(string(out), "\n") {
			if fused.MatchString(l) && line.MatchString(l) && !strings.Contains(l, "_test.go") {
				t.Errorf("%s: fused multiply-add at %s", target.goarch, filepath.Base(line.FindStringSubmatch(l)[1]))
			}
		}
	}
}
