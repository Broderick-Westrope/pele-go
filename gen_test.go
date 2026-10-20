package pele_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/oracle"
)

// genSource is one known source split into the parts the generator recombines.
type genSource struct {
	front  string
	header string
	body   []string
}

func splitSource(src string) genSource {
	var g genSource
	rest := src
	if strings.HasPrefix(rest, "---\n") {
		if i := strings.Index(rest[4:], "\n---\n"); i >= 0 {
			g.front = rest[4 : 4+i]
			rest = rest[4+i+5:]
		}
	}
	lines := strings.Split(rest, "\n")
	i := 0
	for i < len(lines) {
		l := strings.TrimSpace(lines[i])
		if l == "" || strings.HasPrefix(l, "%%") {
			g.header += lines[i] + "\n"
			i++
			continue
		}
		break
	}
	if i < len(lines) {
		g.header += lines[i]
		i++
	}
	g.body = lines[i:]
	return g
}

type generator struct {
	r       *rand.Rand
	big     bool
	sources []genSource
	fronts  []string
	lines   []string
}

func newGenerator(seed uint64, srcs []string) *generator {
	g := &generator{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
	for _, s := range srcs {
		if len(s) > 20000 {
			continue
		}
		if _, err := pele.Render(context.Background(), s, pele.Options{Now: fixedNow}); err != nil {
			continue
		}
		gs := splitSource(s)
		g.sources = append(g.sources, gs)
		if gs.front != "" {
			g.fronts = append(g.fronts, gs.front)
		}
		for _, l := range gs.body {
			if strings.TrimSpace(l) != "" {
				g.lines = append(g.lines, l)
			}
		}
	}
	return g
}

var genLabels = []string{
	"中文标签", "日本語のラベル", "emoji \U0001F600\U0001F680", "Ünïcödé façade", "عربى", "עברית",
	"a & b < c > d", "#quot;quoted#quot;", "&amp;&lt;&#35;", "<b>bold</b> <i>it</i>", "line<br>break",
	"fa:fa-car icon", "`**md** _x_`", "a\"b'c", "x;y", "tab\there", "0", "-1.5e3", "NaN",
	"a very long label that keeps going and going so that wrapping and measuring have to work hard",
	"x", "", " ", "\u200b", "\u00a0nbsp", "ß", "İstanbul", "ﬁ ligature", "\U0001F468\u200d\U0001F469\u200d\U0001F467",
}

func (g *generator) label() string {
	if g.r.IntN(6) == 0 {
		return strings.Repeat(genLabels[g.r.IntN(len(genLabels))]+" ", 1+g.r.IntN(40))
	}
	return genLabels[g.r.IntN(len(genLabels))]
}

var genNumbers = []string{"0", "00", "1", "2", "7", "0.5", "0.0001", "1.005", "99.995", "100", "101", "1000000",
	"123456789012", "1e3", "3.14159265358979", "10000000000000000000000", "0.1", "9", "13", "64", "255", "256"}

var genConfigs = []string{
	"theme: dark", "theme: forest", "theme: neutral", "theme: base", "look: handDrawn", "look: neo",
	"fontSize: 22", "fontSize: 9", "wrap: true", "layout: elk", "htmlLabels: false", "htmlLabels: true",
	"fontFamily: \"Comic Sans MS\"", "darkMode: true", "securityLevel: loose",
	"themeVariables:\n    primaryColor: \"#ff0000\"\n    fontSize: \"20px\"",
	"themeVariables:\n    lineColor: red\n    background: \"#000\"",
}

func (g *generator) front() string {
	var parts []string
	if g.r.IntN(2) == 0 && len(g.fronts) > 0 {
		parts = append(parts, g.fronts[g.r.IntN(len(g.fronts))])
	}
	if g.r.IntN(3) == 0 {
		parts = append(parts, "title: "+strconv.Quote(g.label()))
	}
	if g.r.IntN(2) == 0 {
		var cfg []string
		for range 1 + g.r.IntN(3) {
			cfg = append(cfg, "  "+genConfigs[g.r.IntN(len(genConfigs))])
		}
		parts = append(parts, "config:\n"+strings.Join(cfg, "\n"))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

var (
	genQuoted  = regexp.MustCompile(`"[^"\n]*"`)
	genNumber  = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	genBracket = regexp.MustCompile(`\[[^\]\[\n"]*\]`)
	genIdent   = regexp.MustCompile(`\b[A-Z][A-Za-z0-9_]*\b|\b[a-z]\d?\b`)
)

func (g *generator) mutate(l, suffix, indent string) string {
	l = rename(l, suffix)
	if g.r.IntN(5) == 0 {
		l = genQuoted.ReplaceAllStringFunc(l, func(s string) string {
			if g.r.IntN(2) == 0 {
				return s
			}
			return `"` + strings.ReplaceAll(g.label(), `"`, "") + `"`
		})
	}
	if g.r.IntN(8) == 0 {
		l = genNumber.ReplaceAllStringFunc(l, func(s string) string {
			if g.r.IntN(3) != 0 {
				return s
			}
			return genNumbers[g.r.IntN(len(genNumbers))]
		})
	}
	if g.r.IntN(8) == 0 {
		l = genBracket.ReplaceAllStringFunc(l, func(s string) string {
			if g.r.IntN(2) == 0 {
				return s
			}
			return "[" + strings.NewReplacer("[", "", "]", "", `"`, "").Replace(g.label()) + "]"
		})
	}
	if g.r.IntN(80) == 0 {
		l += " " + g.label()
	}
	return indent + l
}

// generate builds one diagram: front matter, a known header and chunks of known bodies,
// with identifiers renamed per chunk and labels, numbers and indentation mutated.
func (g *generator) generate() string {
	base := g.sources[g.r.IntN(len(g.sources))]
	var b strings.Builder
	if fm := g.front(); fm != "" && base.front == "" || g.r.IntN(4) == 0 {
		if fm == "" {
			fm = base.front
		}
		if fm != "" {
			b.WriteString("---\n" + fm + "\n---\n")
		}
	} else if base.front != "" {
		b.WriteString("---\n" + base.front + "\n---\n")
	}
	b.WriteString(base.header + "\n")
	if g.big {
		return g.generateBig(base, &b)
	}
	chunks := 1 + g.r.IntN(4)
	if g.r.IntN(5) == 0 {
		chunks += g.r.IntN(30)
	}
	for c := range chunks {
		src := base
		if c > 0 || g.r.IntN(3) == 0 {
			src = g.sources[g.r.IntN(len(g.sources))]
		}
		suffix, indent := "", ""
		if c > 0 && g.r.IntN(2) == 0 {
			suffix = "_" + strconv.Itoa(c)
		}
		if c > 0 && g.r.IntN(2) == 0 {
			indent = strings.Repeat(" ", 2*(1+g.r.IntN(3)))
		}
		switch g.r.IntN(3) {
		case 0: // the whole body
			for _, l := range src.body {
				b.WriteString(g.mutate(l, suffix, indent) + "\n")
			}
		case 1: // a window of it
			if len(src.body) > 0 {
				i := g.r.IntN(len(src.body))
				j := i + 1 + g.r.IntN(len(src.body)-i)
				for _, l := range src.body[i:j] {
					b.WriteString(g.mutate(l, suffix, indent) + "\n")
				}
			}
		default: // random lines from anywhere
			if len(g.lines) > 0 {
				for range 1 + g.r.IntN(8) {
					b.WriteString(g.mutate(g.lines[g.r.IntN(len(g.lines))], suffix, indent) + "\n")
				}
			}
		}
	}
	s := b.String()
	if len(s) > 45000 {
		s = s[:45000]
	}
	return s
}

// rename appends suffix to the identifiers of l, leaving calls such as C4's Person( alone.
func rename(l, suffix string) string {
	if suffix == "" {
		return l
	}
	var b strings.Builder
	last := 0
	for _, m := range genIdent.FindAllStringIndex(l, -1) {
		b.WriteString(l[last:m[1]])
		if m[1] >= len(l) || l[m[1]] != '(' {
			b.WriteString(suffix)
		}
		last = m[1]
	}
	b.WriteString(l[last:])
	return b.String()
}

// generateBig repeats the bodies of one to three sources many times, renaming their
// identifiers in each copy, for diagrams with many nodes that stay valid.
func (g *generator) generateBig(base genSource, b *strings.Builder) string {
	picked := []genSource{base}
	for range g.r.IntN(3) {
		picked = append(picked, g.sources[g.r.IntN(len(g.sources))])
	}
	indent := ""
	if g.r.IntN(2) == 0 {
		indent = "  "
	}
	copies := 5 + g.r.IntN(60)
	for c := range copies {
		src := picked[c%len(picked)]
		suffix := ""
		if c > 0 {
			suffix = "_" + strconv.Itoa(c)
		}
		for _, l := range src.body {
			if c > 0 {
				l = indent + l
			}
			l = rename(l, suffix)
			if g.r.IntN(10) == 0 {
				l = genQuoted.ReplaceAllStringFunc(l, func(string) string {
					return `"` + strings.ReplaceAll(g.label(), `"`, "") + `"`
				})
			}
			b.WriteString(l + "\n")
		}
		if b.Len() > 40000 {
			break
		}
	}
	s := b.String()
	if len(s) > 45000 {
		s = s[:45000]
	}
	return s
}

// genDiffers reports how the port's result for src differs from want, or "" when it does not.
func genDiffers(src string, opts pele.Options, want *oracle.Result, wantErr *oracle.Error) (diff string) {
	defer func() {
		if r := recover(); r != nil {
			diff = fmt.Sprintf("panic: %v", r)
		}
	}()
	got, gotErr := core.Render(context.Background(), src, opts)
	if wantErr != nil {
		var pe *pele.Error
		if !errors.As(gotErr, &pe) {
			return fmt.Sprintf("got error %v, want %+v", gotErr, *wantErr)
		}
		g := oracle.Error{Code: string(pe.Code), Message: pe.Message, Type: pe.Type, MermaidType: pe.MermaidType,
			Line: pe.Line, Column: pe.Column, Snippet: pe.Snippet}
		if g != *wantErr {
			return fmt.Sprintf("error\n got %+v\nwant %+v", g, *wantErr)
		}
		return ""
	}
	if gotErr != nil {
		return fmt.Sprintf("got error %v, want success", gotErr)
	}
	if got.SVG != want.SVG {
		i := 0
		for i < len(got.SVG) && i < len(want.SVG) && got.SVG[i] == want.SVG[i] {
			i++
		}
		lo := max(0, i-150)
		return fmt.Sprintf("SVG differs at %d:\n got …%s\nwant …%s", i, clip(got.SVG[lo:], 400), clip(want.SVG[lo:], 400))
	}
	if !sameNumber(got.Width, want.Width) || !sameNumber(got.Height, want.Height) {
		return fmt.Sprintf("size %vx%v, want %vx%v", got.Width, got.Height, want.Width, want.Height)
	}
	if l := oracleLinks(got.Links); !slices.Equal(l, want.Links) {
		return fmt.Sprintf("links %+v, want %+v", l, want.Links)
	}
	return ""
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// TestGeneratedParity renders PELE_GEN structured random diagrams per type against Pele in
// Node, recombining the corpus and captured sources, which mutation fuzzing rarely grows
// into big valid diagrams. PELE_GEN_TYPE limits it to some types (comma separated),
// PELE_GEN_SEED picks the sequence, PELE_GEN_BIG builds many-node diagrams instead, and
// mismatches are written to PELE_GEN_OUT.
func TestGeneratedParity(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("PELE_GEN"))
	if n == 0 {
		t.Skip("PELE_GEN is not set")
	}
	seed, _ := strconv.ParseUint(os.Getenv("PELE_GEN_SEED"), 10, 64)
	out := os.Getenv("PELE_GEN_OUT")
	if out == "" {
		out = t.TempDir()
	}
	byType := corpusByType(t)
	for typ, inputs := range handInputs {
		byType[typ] = append(byType[typ], inputs...)
	}
	types := slices.Sorted(maps.Keys(byType))
	if only := os.Getenv("PELE_GEN_TYPE"); only != "" {
		types = strings.Split(only, ",")
	}
	var mu sync.Mutex
	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			srcs := byType[typ]
			for _, rec := range readShard(t, typ) {
				if src, ok := rec.Get("src"); ok {
					srcs = append(srcs, src.(string))
				}
			}
			srcs = portable(srcs)
			g := newGenerator(seed+uint64(len(typ))*7919+uint64(typ[0]), srcs)
			g.big = os.Getenv("PELE_GEN_BIG") != ""
			zones := zonesOf(typ)
			nodes := map[string]*oracle.Node{}
			for _, zone := range zones {
				nodes[zone] = startNodeIn(t, zone)
			}
			defer func() {
				for _, n := range nodes {
					_ = n.Close()
				}
			}()
			var ok, failed, mismatched int
			for i := range n {
				src := g.generate()
				for try := 0; try < 6 && g.r.IntN(10) < 7; try++ {
					if _, err := pele.Render(context.Background(), src, pele.Options{Now: fixedNow}); err == nil {
						break
					}
					src = g.generate()
				}
				zone := zones[g.r.IntN(len(zones))]
				set := optionSets[0]
				if g.r.IntN(3) == 0 {
					set = optionSets[g.r.IntN(len(optionSets))]
				}
				set = set.in(location(t, zone))
				measurer := ""
				if g.r.IntN(4) == 0 {
					measurer = "length"
				}
				want, wantErr, err := nodes[zone].Render(src, oracle.NodeOptions{Options: set.js, Measurer: measurer, Now: float64(fixedNow.UnixMilli())})
				if err != nil {
					t.Fatalf("node render: %v", err)
				}
				if wantErr != nil {
					failed++
				} else {
					ok++
				}
				var m pele.TextMeasurer
				if measurer == "length" {
					m = lengthMeasurer{}
				}
				if d := genDiffers(src, set.options(m), want, wantErr); d != "" {
					mismatched++
					dir := filepath.Join(out, typ)
					_ = os.MkdirAll(dir, 0o755)
					name := filepath.Join(dir, fmt.Sprintf("%04d", i))
					_ = os.WriteFile(name+".src", []byte(src), 0o644)
					_ = os.WriteFile(name+".txt", []byte(fmt.Sprintf("set=%s measurer=%q zone=%q\n%s\n", set.name, measurer, zone, d)), 0o644)
				}
			}
			mu.Lock()
			fmt.Printf("GEN %s ok=%d err=%d mismatch=%d\n", typ, ok, failed, mismatched)
			mu.Unlock()
		})
	}
}

// TestProbe compares the source in the file PELE_PROBE against Pele in Node.
func TestProbe(t *testing.T) {
	file := os.Getenv("PELE_PROBE")
	if file == "" {
		t.Skip("PELE_PROBE is not set")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	node := startNodeIn(t, os.Getenv("PELE_PROBE_ZONE"))
	defer node.Close()
	measurer := os.Getenv("PELE_PROBE_MEASURER")
	var m pele.TextMeasurer
	if measurer == "length" {
		m = lengthMeasurer{}
	}
	want, wantErr, err := node.Render(string(data), oracle.NodeOptions{Measurer: measurer, Now: float64(fixedNow.UnixMilli())})
	if err != nil {
		t.Fatal(err)
	}
	if d := genDiffers(string(data), pele.Options{Measurer: m, Now: fixedNow}, want, wantErr); d != "" {
		fmt.Println("DIFF", d)
	} else {
		fmt.Println("SAME", wantErr != nil)
	}
}
