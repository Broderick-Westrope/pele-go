package agentflow

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
)

// Ported from upstream's tests/compat/agentflow/conformance.spec.ts and runner.ts, Mermaid's
// agentflow conformance suite: each <case>-agentflow.mmd under testdata/conformance is
// parsed, its graph built to run the validators, and the outcome and diagnostics compared
// with <case>-agentflow.expected.json.

type fixtureResult struct {
	outcome     string
	diagnostics []*diagnostic
	semantic    *jsobj.Object
	parseError  string
}

func runFixture(source string) fixtureResult {
	db, err := parseRaw(source, true)
	if err != nil {
		return fixtureResult{outcome: "parse-error", parseError: err.Message}
	}
	buildAgentGraph(db, 0, nil)
	outcome := "valid"
	for _, d := range db.diagnostics {
		if d.severity == "error" {
			outcome = "error"
			break
		}
		outcome = "warning"
	}
	return fixtureResult{outcome: outcome, diagnostics: db.diagnostics, semantic: db.semanticModel()}
}

func field(o *jsobj.Object, key string) (string, bool) {
	v, ok := o.Get(key)
	if !ok {
		return "", false
	}
	switch v := v.(type) {
	case string:
		return v, true
	case float64:
		return stringify(v), true
	}
	return "", false
}

func diagnosticMatches(actual *diagnostic, expected *jsobj.Object) bool {
	if id, _ := field(expected, "id"); actual.id != id {
		return false
	}
	if nodeID, ok := field(expected, "nodeId"); ok && actual.nodeID != nodeID {
		return false
	}
	if edgeID, ok := field(expected, "edgeId"); ok && actual.edgeID != edgeID {
		return false
	}
	if line, ok := field(expected, "line"); ok && (actual.position == nil || stringify(float64(actual.position.startLine)) != line) {
		return false
	}
	return true
}

func list(o *jsobj.Object, key string) ([]any, bool) {
	v, ok := o.Get(key)
	l, _ := v.([]any)
	return l, ok
}

func matchExpected(result fixtureResult, expected *jsobj.Object) []string {
	var failures []string
	if want, _ := field(expected, "outcome"); result.outcome != want {
		failures = append(failures, `expected outcome "`+want+`", got "`+result.outcome+`" `+result.parseError)
	}
	if want, ok := field(expected, "parseErrorContains"); ok && !strings.Contains(result.parseError, want) {
		failures = append(failures, `expected the parse error to contain "`+want+`", got "`+result.parseError+`"`)
	}
	diagnostics, declared := list(expected, "diagnostics")
	for _, e := range diagnostics {
		if !slices.ContainsFunc(result.diagnostics, func(d *diagnostic) bool { return diagnosticMatches(d, e.(*jsobj.Object)) }) {
			failures = append(failures, "missing diagnostic "+stringify(e))
		}
	}
	// A fixture that declares diagnostics pins the whole set.
	if allow, _ := expected.Get("allowExtraDiagnostics"); declared && allow != true {
		for _, d := range result.diagnostics {
			if !slices.ContainsFunc(diagnostics, func(e any) bool { return diagnosticMatches(d, e.(*jsobj.Object)) }) {
				failures = append(failures, "unexpected diagnostic "+stringify(d.object()))
			}
		}
	}
	assertions, ok := expected.Get("semanticAssertions")
	if !ok {
		return failures
	}
	if result.semantic == nil {
		return append(failures, "semanticAssertions were declared but no semantic model was produced")
	}
	model := result.semantic
	vertices, _ := list(model, "vertices")
	edges, _ := list(model, "edges")
	wantVertices, _ := list(assertions.(*jsobj.Object), "vertices")
	for _, w := range wantVertices {
		want := w.(*jsobj.Object)
		id, _ := field(want, "id")
		i := slices.IndexFunc(vertices, func(v any) bool { got, _ := field(v.(*jsobj.Object), "id"); return got == id })
		if i < 0 {
			failures = append(failures, `expected vertex "`+id+`" not found in semantic model`)
			continue
		}
		actual := vertices[i].(*jsobj.Object)
		if kind, ok := field(want, "vertexKind"); ok {
			if got, _ := field(actual, "vertexKind"); got != kind {
				failures = append(failures, `vertex "`+id+`" expected vertexKind="`+kind+`" but got "`+got+`"`)
			}
		}
		if wantMeta, ok := want.Get("metadata"); ok {
			gotMeta, _ := actual.Get("metadata")
			gm, _ := gotMeta.(*jsobj.Object)
			for _, key := range wantMeta.(*jsobj.Object).Keys() {
				wv, _ := wantMeta.(*jsobj.Object).Get(key)
				gv, _ := gm.Get(key)
				if stringify(gv) != stringify(wv) {
					failures = append(failures, `vertex "`+id+`" metadata.`+key+` expected `+stringify(wv)+` but got `+stringify(gv))
				}
			}
		}
	}
	wantEdges, _ := list(assertions.(*jsobj.Object), "edges")
	for _, w := range wantEdges {
		want := w.(*jsobj.Object)
		i := slices.IndexFunc(edges, func(e any) bool {
			actual := e.(*jsobj.Object)
			if id, ok := field(want, "id"); ok {
				got, _ := field(actual, "id")
				return got == id
			}
			start, hasStart := field(want, "start")
			end, hasEnd := field(want, "end")
			gotStart, _ := field(actual, "start")
			gotEnd, _ := field(actual, "end")
			return (hasStart || hasEnd) && (!hasStart || gotStart == start) && (!hasEnd || gotEnd == end)
		})
		if i < 0 {
			failures = append(failures, "expected edge "+stringify(want)+" not found in semantic model")
			continue
		}
		if semantic, ok := field(want, "edgeSemantic"); ok {
			if got, _ := field(edges[i].(*jsobj.Object), "edgeSemantic"); got != semantic {
				failures = append(failures, "edge "+stringify(want)+` expected edgeSemantic="`+semantic+`" but got "`+got+`"`)
			}
		}
	}
	return failures
}

func TestConformance(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join("testdata", "conformance", "*-agentflow.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) == 0 {
		t.Fatal("no fixtures")
	}
	for _, path := range sources {
		name := strings.TrimSuffix(filepath.Base(path), "-agentflow.mmd")
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(strings.TrimSuffix(path, ".mmd") + ".expected.json")
			if err != nil {
				t.Fatal(err)
			}
			expected, err := jsjson.Parse(string(data))
			if err != nil {
				t.Fatal(err)
			}
			result := runFixture(string(source))
			for _, failure := range matchExpected(result, expected.(*jsobj.Object)) {
				t.Error(failure)
			}
		})
	}
}
