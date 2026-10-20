package agentflow

import (
	"slices"
	"testing"
)

// Ported from upstream's tests/compat/agentflow/agentflow-global.spec.ts: a `global` block
// keeps the nodes it names out of every flow.

type globalCase struct {
	src string
	// flow is the flow to look at, "" for the only one.
	flow    string
	members []string
	// parents are the graph parents by node, "" for none.
	parents map[string]string
}

func TestGlobalBlocks(t *testing.T) {
	tests := map[string]globalCase{
		"before the flow": {
			src:     "agentflow-beta\n\nglobal\n  A\nend\n\nflow\n A --> B --> C\nend",
			members: []string{"C", "B"},
			parents: map[string]string{"A": "", "B": "subGraph0", "C": "subGraph0"},
		},
		"after the flow": {
			src:  "agentflow-beta\n\nflow pipeline[\"Pipeline\"]\n  A --> B\nend\n\nglobal\n  A\nend",
			flow: "pipeline", members: []string{"B"},
			parents: map[string]string{"A": "", "B": "pipeline"},
		},
		"edges inside": {
			src:  "agentflow-beta\n\nglobal\n  A --> X\nend\n\nflow f[\"Flow\"]\n  A --> B\n  X --> B\nend",
			flow: "f", members: []string{"B"},
			parents: map[string]string{"A": "", "X": ""},
		},
		"nested in a flow": {
			src:  "agentflow-beta\n\nflow f[\"Flow\"]\n  global\n    A\n  end\n  A --> B\nend",
			flow: "f", members: []string{"B"},
			parents: map[string]string{"A": "", "B": "f"},
		},
		"several blocks": {
			src:  "agentflow-beta\n\nglobal\n  A\nend\n\nglobal\n  B\nend\n\nflow f[\"Flow\"]\n  A --> B --> C\nend",
			flow: "f", members: []string{"C"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db := rawModel(t, tc.src)
			sg := db.subgraphs[0]
			if tc.flow != "" {
				sg = db.subgraph(tc.flow)
			}
			if !slices.Equal(sg.nodes, tc.members) {
				t.Errorf("members %v, want %v", sg.nodes, tc.members)
			}
			graph := buildAgentGraph(db, 0, nil)
			for id, parent := range tc.parents {
				if n := nodeByID(graph, id); n == nil || n.parentID != parent || n.hasParent != (parent != "") {
					t.Errorf("parent of %s: %+v, want %q", id, n, parent)
				}
			}
			var groups []string
			for _, n := range graph.nodes {
				if n.isGroup {
					groups = append(groups, n.id)
				}
			}
			if len(groups) != len(db.subgraphs) {
				t.Errorf("groups %v", groups)
			}
		})
	}
}

func TestGlobalBlockKeepsLabelsShapesAndEdges(t *testing.T) {
	db := rawModel(t, "agentflow-beta\n\nglobal\n  A[\"Shared input\"]\n  D{\"Decision\"}\nend\n\nflow f[\"Flow\"]\n  A --> B --> D\nend")
	graph := buildAgentGraph(db, 0, nil)
	if a := nodeByID(graph, "A"); a.hasParent || a.label != "Shared input" {
		t.Errorf("A %+v", a)
	}
	if d := nodeByID(graph, "D"); d.hasParent || d.shape != "diamond" {
		t.Errorf("D %+v", d)
	}
	db = rawModel(t, "agentflow-beta\n\nglobal\n  A\nend\n\nflow\n A --> B --> C\nend")
	graph = buildAgentGraph(db, 0, nil)
	for _, pair := range [][2]string{{"A", "B"}, {"B", "C"}} {
		if !slices.ContainsFunc(graph.edges, func(e *graphEdge) bool { return e.start == pair[0] && e.end == pair[1] }) {
			t.Errorf("no edge %v", pair)
		}
	}
	db = rawModel(t, "agentflow-beta\n\nflow f[\"global settings\"]\n  a[\"a global thing\"] --> b\nend")
	if db.subgraphs[0].title != "global settings" {
		t.Errorf("title %q", db.subgraphs[0].title)
	}
	if a := nodeByID(buildAgentGraph(db, 0, nil), "a"); a.label != "a global thing" {
		t.Errorf("label %q", a.label)
	}
}
