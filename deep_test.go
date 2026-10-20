package pele_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	pele "github.com/Broderick-Westrope/pele-go"
)

// TestDeepNestingRenders renders sources nested 20,000 deep. Upstream walks all of them
// without recursion, so V8 renders each (checked against Node up to 100,000 levels); the
// port must too, rather than throw the limit error or overflow its stack.
func TestDeepNestingRenders(t *testing.T) {
	const depth = 20000
	nest := func(head, open, middle, close string) string {
		var b strings.Builder
		b.WriteString(head)
		for i := range depth {
			b.WriteString(strings.ReplaceAll(open, "#", strconv.Itoa(i)))
		}
		b.WriteString(middle)
		b.WriteString(strings.Repeat(close, depth))
		return b.String()
	}
	tests := map[string]string{
		"flowchart subgraphs":  nest("flowchart TD\n", "subgraph s#\n", "a\n", "end\n"),
		"agentflow flows":      nest("agentflow-beta TB\n", "flow f#[\"F\"]\n", "a[\"A\"]\n", "end\n"),
		"state composites":     nest("stateDiagram-v2\n", "state s# {\n", "a\n", "}\n"),
		"block blocks":         nest("block\n", "block:b#\n", "a\n", "end\n"),
		"c4 boundaries":        nest("C4Context\n", "Boundary(b#, \"B\") {\n", "Person(p, \"P\")\n", "}\n"),
		"sequence loops":       nest("sequenceDiagram\n", "loop L\n", "A->>B: m\n", "end\n"),
		"railroad groups":      nest("railroad-ebnf-beta\nr = ", "( ", `"a"`, " )") + " ;\n",
		"railroad sequences":   nest("railroad-beta\nr = ", "sequence(", `terminal("a")`, ")") + " ;\n",
		"directive json":       nest("%%{init: ", `{"a":`, "1", "}") + "}%%\ninfo\n",
		"usecase json":         nest("usecase-beta\njson S@{ ", `"a": {`, `"b": 1`, "}") + " }\n",
		"info title tags":      nest("info\ntitle ", "<b>", "x", "</b>") + "\n",
		"class generic params": nest("classDiagram\nclass A {\n+f() List", "~L", "", "~") + "\n}\n",
	}
	limit, outputLimit := 1e7, 1e9
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := pele.Render(context.Background(), src, pele.Options{Limit: &limit, OutputLimit: &outputLimit})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
		})
	}
}
