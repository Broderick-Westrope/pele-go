package agentflow

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
)

// Ported from upstream's tests/compat/agentflow specs: agentflow-metadata-trailing-comma,
// agentflow-empty-metadata, agentflow-blank-line-fold and agentflow-diagnostics-lifecycle.

func rawModel(t *testing.T, text string) *agentflowDb {
	t.Helper()
	db, err := parseRaw(text, true)
	if err != nil {
		t.Fatalf("parse: %s", err.Message)
	}
	return db
}

func hasFields(t *testing.T, got *jsobj.Object, want map[string]string) {
	t.Helper()
	for key, value := range want {
		v, _ := got.Get(key)
		if stringify(v) != value {
			t.Errorf("%s = %s, want %s", key, stringify(v), value)
		}
	}
}

func firstConnector(db *agentflowDb) *agentNode {
	for _, c := range db.connectors.All() {
		return c
	}
	return nil
}

func TestTrailingCommasInMultilineMetadata(t *testing.T) {
	connector := func(src string) *jsobj.Object { return firstConnector(rawModel(t, src)).metadata }
	vertexMeta := func(src, id string) *jsobj.Object {
		v, _ := rawModel(t, src).nodes.Get(id)
		return v.metadata
	}
	hasFields(t, connector("agentflow-beta TB\n  connector github[\"GitHub\"]@{\n    protocol: \"mcp\",\n    transport: \"stdio\",\n  }"),
		map[string]string{"protocol": `"mcp"`, "transport": `"stdio"`})
	hasFields(t, connector("agentflow-beta TB\n  connector github[\"GitHub\"]@{\n    protocol: \"mcp\",\n    endpoint: \"https://api.github.com\"\n    token_required: true,\n  }"),
		map[string]string{"protocol": `"mcp"`, "endpoint": `"https://api.github.com"`, "token_required": "true"})
	hasFields(t, vertexMeta("agentflow-beta TB\n  a[\"Alpha\"]\n  a@{\n    description: \"alpha node\",\n    instruction: \"be careful\",\n  }", "a"),
		map[string]string{"description": `"alpha node"`, "instruction": `"be careful"`})
	flow := rawModel(t, "agentflow-beta TB\n  flow f[\"Flow\"]@{\n    model: \"claude-opus-4\",\n    memory: \"shared\",\n  }\n    a --> b\n  end")
	hasFields(t, flow.subgraph("f").metadata, map[string]string{"model": `"claude-opus-4"`, "memory": `"shared"`})
	hasFields(t, connector("agentflow-beta TB\n  connector github[\"GitHub\"]@{\n    protocol: \"mcp\", # main protocol\n    transport: \"stdio\"\n  }"),
		map[string]string{"protocol": `"mcp"`, "transport": `"stdio"`})
	hasFields(t, vertexMeta("agentflow-beta TB\n  a[\"Alpha\"]\n  a@{\n    instruction: |\n      Do a thing,\n      then stop\n    protocol: \"mcp\",\n  }", "a"),
		map[string]string{"instruction": `"Do a thing,\nthen stop\n"`, "protocol": `"mcp"`})
	hasFields(t, vertexMeta("agentflow-beta TB\n  a[\"Alpha\"]\n  a@{\n    tools: [\"read\",\n      \"write\"],\n    protocol: \"mcp\"\n  }", "a"),
		map[string]string{"tools": `["read","write"]`, "protocol": `"mcp"`})
	hasFields(t, vertexMeta("agentflow-beta TB\n  a[\"Alpha\"]\n  a@{\n    note: 'first,\n      second',\n    protocol: \"mcp\"\n  }", "a"),
		map[string]string{"note": `"first, second"`, "protocol": `"mcp"`})

	full := rawModel(t, "agentflow-beta LR\n  connector github[\"GitHub API\"]@{\n    protocol: \"http\",\n    endpoint: \"https://api.github.com\",\n"+
		"    token_required: true,\n  }\n\n  query[\"query\"]\n  query@{ shape: input }\n  fetch_issues[\"fetch_issues\"]\n"+
		"  fetch_issues@{ shape: tool, connectorRef: \"github.list_issues\" }\n  query --> fetch_issues")
	hasFields(t, firstConnector(full).metadata, map[string]string{"protocol": `"http"`, "token_required": "true"})
	fetch, _ := full.nodes.Get("fetch_issues")
	hasFields(t, fetch.metadata, map[string]string{"connectorRef": `"github.list_issues"`})
	if len(full.edges) != 1 {
		t.Errorf("%d edges", len(full.edges))
	}
	hasFields(t, connector("agentflow-beta TB\n  connector github[\"GitHub\"]@{ protocol: \"mcp\", }"), map[string]string{"protocol": `"mcp"`})
}

func TestEmptyMetadataBlocks(t *testing.T) {
	built := func(src string) *agentflowDb {
		db := rawModel(t, src)
		buildAgentGraph(db, 0, nil)
		return db
	}
	db := built("agentflow-beta TB\n  newtask[\"Example\"]@{\n}")
	if v, ok := db.nodes.Get("newtask"); !ok || v.text != "Example" {
		t.Errorf("newtask %+v", v)
	}
	for _, src := range []string{"agentflow-beta TB\n  a[\"A\"]@{\n\n}", "agentflow-beta TB\n  a[\"A\"]@{   }"} {
		if _, ok := built(src).nodes.Get("a"); !ok {
			t.Errorf("no a in %q", src)
		}
	}
	single, _ := built("agentflow-beta TB\n  a[\"A\"]@{}").nodes.Get("a")
	multi, _ := built("agentflow-beta TB\n  a[\"A\"]@{\n}").nodes.Get("a")
	if stringify(single.metadata) != stringify(multi.metadata) || single.typ != multi.typ {
		t.Errorf("single %s %q, multi %s %q", stringify(single.metadata), single.typ, stringify(multi.metadata), multi.typ)
	}
	singleFlow := built("agentflow-beta TB\n  flow f[\"F\"]\n    a --> b\n  end\n  f@{}")
	multiFlow := built("agentflow-beta TB\n  flow f[\"F\"]\n    a --> b\n  end\n  f@{\n}")
	if multiFlow.subgraph("f") == nil || stringify(multiFlow.subgraph("f").metadata) != stringify(singleFlow.subgraph("f").metadata) {
		t.Error("flow metadata differs")
	}
	_, inSingle := singleFlow.nodes.Get("f")
	_, inMulti := multiFlow.nodes.Get("f")
	if inSingle != inMulti {
		t.Errorf("vertex f: single %v, multi %v", inSingle, inMulti)
	}

	if _, err := parseRaw("agentflow-beta TB\n  a[\"A\"]@{ shape: 123 }", true); err == nil || !strings.Contains(err.Message, "No such shape") {
		t.Errorf("shape 123: %v", err)
	}
	if v, _ := built("agentflow-beta TB\n  a[\"A\"]@{ shape: task }").nodes.Get("a"); v.typ != "roundedRect" {
		t.Errorf("task is %q", v.typ)
	}
}

func elementByID(db *agentflowDb, id string) *elementMapping {
	for _, m := range db.mappings {
		if m.id == id {
			return m
		}
	}
	return nil
}

func TestBlankLinesAfterMetadataDoNotShiftPositions(t *testing.T) {
	tests := map[string]struct {
		src  string
		id   string
		line int
	}{
		"one blank line":    {src: "agentflow-beta\nflow f[\"F\"]\n  a[\"A\"]@{ shape: rounded }\n\n  b[\"B\"]\nend", id: "b", line: 5},
		"two blank lines":   {src: "agentflow-beta\nflow f[\"F\"]\n  a[\"A\"]@{ shape: rounded }\n\n\n  b[\"B\"]\nend", id: "b", line: 6},
		"a comment line":    {src: "agentflow-beta\nflow f[\"F\"]\n  a[\"A\"]@{ shape: rounded }\n  %% a note\n  b[\"B\"]\nend", id: "b", line: 5},
		"trailing blanks":   {src: "agentflow-beta TB\n  a[\"A\"]@{ shape: rounded }   \n\n  b[\"B\"]", id: "b", line: 4},
		"an edge after one": {src: "agentflow-beta TB\n  a[\"A\"]@{ shape: rounded }\n\n  c[\"C\"]\n  a --> c", id: "c", line: 5},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db, err := parseRaw(tc.src, false)
			if err != nil {
				t.Fatal(err.Message)
			}
			var m *elementMapping
			if name == "an edge after one" {
				for _, e := range db.mappings {
					if e.typ == "edge" {
						m = e
						break
					}
				}
			} else {
				m = elementByID(db, tc.id)
			}
			if m == nil || m.position.startLine != tc.line {
				t.Fatalf("mapping %+v, want line %d", m, tc.line)
			}
		})
	}
	_, err := parseRaw("agentflow-beta TB\n  a[\"A\"]@{ shape: rounded }\n\n  b[\"B\"]@{ shape: rounded }\n\n  c[\"C\"]\"]\n", false)
	if err == nil {
		t.Fatal("no error")
	}
	if m := regexp.MustCompile(`[Pp]arse error on line (\d+)`).FindStringSubmatch(err.Message); m == nil || m[1] != "6" {
		t.Errorf("error %q", err.Message)
	}
}

func TestDiagnosticsAreRaisedByBuildingTheGraphOnce(t *testing.T) {
	unsupported := "agentflow-beta TB\n  a[\"Alpha\"]\n  a@{ shape: \"triangle\" }\n  b[\"Beta\"]\n  a --> b"
	removed := "agentflow-beta TB\n  a[\"Alpha\"]\n  a@{ shape: \"cylinder\" }\n  b[\"Beta\"]\n  a --> b"
	find := func(db *agentflowDb, id string) []*diagnostic {
		var out []*diagnostic
		for _, d := range db.diagnostics {
			if d.id == id {
				out = append(out, d)
			}
		}
		return out
	}
	db := rawModel(t, unsupported)
	for range 3 {
		buildAgentGraph(db, 0, nil)
	}
	got := find(db, "SHAPE_UNSUPPORTED")
	if len(got) != 1 || got[0].nodeID != "a" || got[0].severity != "warning" || got[0].position == nil || got[0].position.startLine <= 0 {
		t.Errorf("unsupported: %+v", got)
	}
	db = rawModel(t, removed)
	buildAgentGraph(db, 0, nil)
	if got := find(db, "SHAPE_REMOVED"); len(got) != 1 || got[0].severity != "error" {
		t.Errorf("removed: %+v", got)
	}
}
