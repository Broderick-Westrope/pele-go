package wardley

import "testing"

// Ported from tests/compat/wardley/wardleyBuilder.spec.ts.
func TestResolveNodeID(t *testing.T) {
	tests := map[string]struct {
		nodes []nodeUpdate
		name  string
		want  string
	}{
		"exact id": {
			nodes: []nodeUpdate{{id: "API", label: "API", x: 60, y: 70}},
			name:  "API",
			want:  "API",
		},
		"label of a pipeline-style synthetic id": {
			nodes: []nodeUpdate{{id: "Kettle_Campfire Kettle", label: "Campfire Kettle", x: 35, y: 45, className: "pipeline-component"}},
			name:  "Campfire Kettle",
			want:  "Kettle_Campfire Kettle",
		},
		"no match": {
			nodes: []nodeUpdate{{id: "API", label: "API", x: 60, y: 70}},
			name:  "Unknown",
			want:  "Unknown",
		},
		"id wins over label": {
			nodes: []nodeUpdate{{id: "Alpha", label: "Beta"}, {id: "Beta", label: "Gamma", x: 1, y: 1}},
			name:  "Beta",
			want:  "Beta",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var b wardleyBuilder
			for _, n := range tc.nodes {
				b.addNode(n)
			}
			if got := b.resolveNodeID(tc.name); got != tc.want {
				t.Errorf("resolveNodeID(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// A node added again keeps its class and label offsets unless the new one sets them, and an
// anchor keeps a component's inertia and strategy.
func TestAddNodeMerges(t *testing.T) {
	var b wardleyBuilder
	b.addNode(nodeUpdate{id: "A", label: "A", className: "component", labelOffsetX: new(3.0), inertia: new(true), sourceStrategy: new("buy")})
	b.startPipeline("A")
	b.addNode(nodeUpdate{id: "A", label: "A", x: 1, y: 2, className: "anchor"})
	n, _ := b.getNode("A")
	if n.className != "anchor" || n.labelOffsetX == nil || *n.labelOffsetX != 3 || !n.inertia || n.sourceStrategy != "buy" || !n.isPipelineParent || n.x != 1 {
		t.Errorf("merged node = %+v", *n)
	}
}
