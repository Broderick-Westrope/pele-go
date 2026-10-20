package agentflow

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
)

// Ported from upstream's tests/agentflow/reference.test.ts: Pele's agentflow model against
// what Mermaid 12.1.0 itself built for the same text, taken from Mermaid's browser build:
// nodes, edges, flows, classes, the layout data with shapes, parents and palette slots,
// element positions, diagnostics and the semantic model.

// lenientYAML matches js-yaml's errors for an alias and for a lone colon in metadata. Pele's
// YAML reader has no aliases and takes both as text, so it accepts these diagrams where
// Mermaid does not.
var lenientYAML = regexp.MustCompile(`alias|explicit mapping pair`)

type reference struct {
	src   string
	model any
	err   string
}

func loadReference(t *testing.T) []reference {
	t.Helper()
	data, err := os.ReadFile("../../internal/oracle/testdata/corpus/agentflow-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jsjson.Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	var out []reference
	for _, entry := range parsed.([]any) {
		o := entry.(*jsobj.Object)
		src, _ := o.Get("src")
		model, _ := o.Get("model")
		msg, _ := o.Get("error")
		e, _ := msg.(string)
		out = append(out, reference{src: src.(string), model: model, err: e})
	}
	return out
}

func TestReferenceHasEnoughToCompare(t *testing.T) {
	refs := loadReference(t)
	models := 0
	for _, r := range refs {
		if r.model != nil {
			models++
		}
	}
	if len(refs) <= 200 || models <= 150 {
		t.Fatalf("%d references, %d with a model", len(refs), models)
	}
}

func TestReferenceAcceptsAndRejectsTheSameInputs(t *testing.T) {
	for _, ref := range loadReference(t) {
		_, err := parseSource(ref.src)
		mine := ""
		if err != nil {
			mine = strings.SplitN(err.Message, "\n", 2)[0]
		}
		switch {
		case ref.err != "" && lenientYAML.MatchString(ref.err):
			if err != nil {
				t.Errorf("Mermaid: %s; Pele rejects it too: %s; %q", ref.err, mine, ref.src)
			}
		case (err == nil) != (ref.model != nil):
			t.Errorf("Mermaid: %q; Pele: %q; %q", ref.err, mine, ref.src)
		}
	}
}

func TestReferenceBuildsTheSameModel(t *testing.T) {
	for _, ref := range loadReference(t) {
		if ref.model == nil {
			continue
		}
		db, err := parseSource(ref.src)
		if err != nil {
			t.Errorf("%q: %s", ref.src, err.Message)
			continue
		}
		if d := difference("model", comparable(project(db)), comparable(ref.model)); d != "" {
			t.Errorf("%q\n%s", ref.src, d)
		}
	}
}
