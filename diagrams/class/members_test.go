package class

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/members.json is upstream's ClassMember run on the inputs of Mermaid's
// classTypes.spec.ts, and a few more.
func TestClassMemberMatchesUpstream(t *testing.T) {
	data, err := os.ReadFile("testdata/members.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input, Type, MemberType, DisplayText, CSSStyle string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		m := newClassMember(tc.Input, tc.Type == "method")
		gotType := "attribute"
		if m.method {
			gotType = "method"
		}
		text, css := m.getDisplayDetails()
		if gotType != tc.MemberType || text != tc.DisplayText || css != tc.CSSStyle {
			t.Errorf("%q as %s: got %s %q %q, want %s %q %q", tc.Input, tc.Type, gotType, text, css, tc.MemberType, tc.DisplayText, tc.CSSStyle)
		}
	}
}
