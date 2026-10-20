package layout_test

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/layout"
)

// num is a number from the fixture, where values JSON cannot hold, and -0, are strings.
type num float64

func (n *num) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		switch s {
		case "Infinity":
			*n = num(math.Inf(1))
		case "-Infinity":
			*n = num(math.Inf(-1))
		case "NaN":
			*n = num(math.NaN())
		case "-0":
			*n = num(math.Copysign(0, -1))
		default:
			return fmt.Errorf("unexpected number %q", s)
		}
		return nil
	}
	v, err := strconv.ParseFloat(string(b), 64)
	*n = num(v)
	return err
}

// The generator writes inputs positionally: see encodeOpt, encodeCNodes and the others in
// scripts/gen-layout-cases.mjs.
type fixtureInput struct {
	Nodes   [][]num `json:"nodes"`
	Edges   [][]num `json:"edges"`
	Dir     string  `json:"dir"`
	Opt     []num   `json:"opt"`
	LanePad num     `json:"lanePad"`
}

type fixtureCase struct {
	Kind   string          `json:"kind"`
	Input  json.RawMessage `json:"input"`
	Output *string         `json:"output"`
	SHA256 string          `json:"sha256"`
}

func options(o []num) layout.LayeredOptions {
	return layout.LayeredOptions{
		NodeSep: float64(o[0]), EdgeSep: float64(o[1]), RankSep: float64(o[2]), PortSep: float64(o[3]),
		Tracks: o[4] == 1, Straight: o[5] == 1, Apart: o[6] == 1, HeadRoom: float64(o[7]),
	}
}

var dirCodes = []layout.Dir{"", layout.TB, layout.BT, layout.LR, layout.RL}

func compoundGraph(in fixtureInput) ([]layout.CNode, []layout.CEdge) {
	nodes := make([]layout.CNode, len(in.Nodes))
	for i, r := range in.Nodes {
		nodes[i] = layout.CNode{
			W: float64(r[0]), H: float64(r[1]), Parent: int(r[2]), IsGroup: r[3] == 1, Dir: dirCodes[int(r[4])],
			PadX: float64(r[5]), PadTop: float64(r[6]), PadBottom: float64(r[7]), MinW: float64(r[8]),
			Seq: float64(r[9]), Span: float64(r[10]), X: float64(r[11]), Y: float64(r[12]),
		}
	}
	edges := make([]layout.CEdge, len(in.Edges))
	for i, r := range in.Edges {
		edges[i] = layout.CEdge{
			Src: int(r[0]), Dst: int(r[1]), Minlen: float64(r[2]), LabelW: float64(r[3]), LabelH: float64(r[4]),
			Route: []float64{}, LabelX: float64(r[5]), LabelY: float64(r[6]),
		}
	}
	return nodes, edges
}

func layeredGraph(in fixtureInput) ([]layout.LNode, []layout.LEdge) {
	nodes := make([]layout.LNode, len(in.Nodes))
	for i, r := range in.Nodes {
		nodes[i] = layout.NewLNode(float64(r[0]), float64(r[1]), layout.Kind(r[2]))
		nodes[i].Pin = float64(r[3])
		nodes[i].Span = float64(r[4])
	}
	edges := make([]layout.LEdge, len(in.Edges))
	for i, r := range in.Edges {
		edges[i] = layout.NewLEdge(int(r[0]), int(r[1]), float64(r[2]), float64(r[3]), float64(r[4]))
		edges[i].TailDx = float64(r[5])
		edges[i].HeadDx = float64(r[6])
		edges[i].TailFixed = r[7] == 1
		edges[i].HeadFixed = r[8] == 1
	}
	return nodes, edges
}

// readCases reads testdata/cases.jsonl.gz, which scripts/gen-layout-cases.mjs writes by running
// upstream's layout under Node.
func readCases(t *testing.T) []fixtureCase {
	t.Helper()
	f, err := os.Open("testdata/cases.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []fixtureCase
	scanner := bufio.NewScanner(z)
	scanner.Buffer(nil, 1<<26)
	for scanner.Scan() {
		var c fixtureCase
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return cases
}

// format writes a number the way the fixture generator does: as String(n), with -0 kept.
func format(v float64) string {
	if v == 0 && math.Signbit(v) {
		return "-0"
	}
	return jsnum.Format(v)
}

type describer struct{ parts []string }

func (d *describer) num(vs ...float64) {
	for _, v := range vs {
		d.parts = append(d.parts, format(v))
	}
}

func (d *describer) list(vs []float64) {
	d.parts = append(d.parts, strconv.Itoa(len(vs)))
	d.num(vs...)
}

func (d *describer) String() string { return strings.Join(d.parts, " ") }

func describeLayered(nodes []layout.LNode, edges []layout.LEdge, r layout.LayeredResult) string {
	var d describer
	d.num(r.Width, r.Height)
	for _, n := range nodes {
		d.num(n.X, n.Y, float64(n.Rank))
	}
	for _, e := range edges {
		if e.Reversed {
			d.parts = append(d.parts, "r")
		} else {
			d.parts = append(d.parts, "f")
		}
		d.num(e.TailDx, e.HeadDx, e.LabelX, e.LabelY)
		d.list(e.Points)
	}
	return d.String()
}

func describeCompound(nodes []layout.CNode, edges []layout.CEdge, r layout.CompoundResult) string {
	var d describer
	d.num(r.Width, r.Height)
	for _, n := range nodes {
		d.num(n.X, n.Y, n.W, n.H, float64(n.Parent), n.Seq, n.PadTop)
	}
	for _, e := range edges {
		d.num(float64(e.Src), float64(e.Dst), e.LabelX, e.LabelY)
		d.list(e.Route)
	}
	return d.String()
}

// run lays a fixture case out and describes the result as the generator does.
func run(t *testing.T, ctx context.Context, c fixtureCase) string {
	t.Helper()
	var in fixtureInput
	if err := json.Unmarshal(c.Input, &in); err != nil {
		t.Fatal(err)
	}
	switch c.Kind {
	case "layered":
		nodes, edges := layeredGraph(in)
		r := layout.Layered(ctx, nodes, edges, options(in.Opt))
		return describeLayered(nodes, edges, r)
	case "compound":
		nodes, edges := compoundGraph(in)
		r := layout.CompoundLayout(ctx, nodes, edges, layout.Dir(in.Dir), options(in.Opt))
		return describeCompound(nodes, edges, r)
	case "lanes":
		nodes, edges := compoundGraph(in)
		r := layout.LaneLayout(ctx, nodes, edges, layout.Dir(in.Dir), options(in.Opt), float64(in.LanePad))
		return describeCompound(nodes, edges, r)
	}
	t.Fatalf("unknown kind %q", c.Kind)
	return ""
}

// firstDifference shows where two descriptions part, with a little of what comes before.
func firstDifference(got, want string) string {
	g := strings.Fields(got)
	w := strings.Fields(want)
	for i := 0; i < min(len(g), len(w)); i++ {
		if g[i] != w[i] {
			from := max(0, i-6)
			return fmt.Sprintf("token %d: got %q, want %q\n  got  ...%s\n  want ...%s", i, g[i], w[i], strings.Join(g[from:min(len(g), i+6)], " "), strings.Join(w[from:min(len(w), i+6)], " "))
		}
	}
	return fmt.Sprintf("got %d tokens, want %d", len(g), len(w))
}

// TestMatchesV8 replays every layout in testdata/cases.jsonl.gz through the port and requires
// the result upstream gave under V8, to the bit.
func TestMatchesV8(t *testing.T) {
	cases := readCases(t)
	counts := map[string]int{}
	for i, c := range cases {
		if c.Kind == "hypot" {
			continue
		}
		counts[c.Kind]++
		name := fmt.Sprintf("%s/%d", c.Kind, i)
		t.Run(name, func(t *testing.T) {
			got := run(t, context.Background(), c)
			if c.Output != nil {
				if got != *c.Output {
					t.Errorf("layout differs from upstream: %s\ninput: %s", firstDifference(got, *c.Output), c.Input)
				}
				return
			}
			sum := sha256.Sum256([]byte(got))
			if hex.EncodeToString(sum[:16]) != c.SHA256 {
				t.Errorf("layout differs from upstream; regenerate the fixture with PELE_LAYOUT_FULL=1 to see where\ninput: %s", c.Input)
			}
		})
	}
	t.Logf("cases: %v", counts)
}
