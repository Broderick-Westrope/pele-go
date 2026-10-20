package treemap

import (
	"encoding/json"
	"math"
	"testing"
)

// formatCases is what upstream's formatter returned under Node for each specifier, for the
// values in formatValues.
const formatCases = `{",":["0","0","1,234,567.891","-0.5","2.5","1e+21","0.000123","NaN","In,fin,ity","100"],"$0,0":["$0","$0","$1,234,567.891","-$0.5","$2.5","$1e+21","$0.000123","$NaN","$In,fin,ity","$100"],"$,.2f":["$0.00","$0.00","$1,234,567.89","-$0.50","$2.50","$1e+21","$0.00","$NaN","$In,fin,ity","$100.00"],".1%":["0.0%","0.0%","123456789.1%","-50.0%","250.0%","1e+23%","0.0%","NaN%","Infinity%","10000.0%"],"d":["0","0","1234568","-1","3","1e+21","0","NaN","Infinity","100"],",.3e":["0.000e+0","0.000e+0","1.235e+6","-5.000e-1","2.500e+0","1.000e+21","1.230e-4","NaN","In,fin,ity","1.000e+2"],"+,":["+0","+0","+1,234,567.891","-0.5","+2.5","+1e+21","+0.000123","+NaN","+In,fin,ity","+100"],"~f":["0","0","1234567.891","-0.5","2.5","1e+21","0.000123","NaN","Infinity","100"],",.2~f":["0","0","1,234,567.89","-0.5","2.5","1e+21","0","NaN","In,fin,ity","100"],"x>+$010,.3f":["+$0.000","+$0.000","+$1,234,567.891","-$0.500","+$2.500","+$1e+21","+$0.000","+$NaN","+$In,fin,ity","+$100.000"],">>,":["0","0","1,234,567.891","-0.5","2.5","1e+21","0.000123","NaN","In,fin,ity","100"],"(":["0","0","1234567.891","-0.5","2.5","1e+21","0.000123","NaN","Infinity","100"],".25f":["0.00000000000000000000","0.00000000000000000000","1234567.89100000006146728992","-0.50000000000000000000","2.50000000000000000000","1e+21","0.00012300000000000001","NaN","Infinity","100.00000000000000000000"],"q":["0","0","1234567.891","-0.5","2.5","1e+21","0.000123","NaN","Infinity","100"],"€^.1f":["0.0","0.0","1234567.9","-0.5","2.5","1e+21","0.0","NaN","Infinity","100.0"],".0f":["0","0","1234568","-1","3","1e+21","0","NaN","Infinity","100"]}`

var formatValues = []float64{0, math.Copysign(0, -1), 1234567.891, -0.5, 2.5, 1e21, 0.000123, math.NaN(), math.Inf(1), 100}

func TestFormatterMatchesV8(t *testing.T) {
	var cases map[string][]string
	if err := json.Unmarshal([]byte(formatCases), &cases); err != nil {
		t.Fatal(err)
	}
	for spec, want := range cases {
		t.Run(spec, func(t *testing.T) {
			format := formatter(spec)
			for i, v := range formatValues {
				if got := format(v); got != want[i] {
					t.Errorf("formatter(%q)(%v) = %q, want %q", spec, v, got, want[i])
				}
			}
		})
	}
}

// Ported from tests/compat/treemap/utils.test.ts.
func TestBuildHierarchy(t *testing.T) {
	roots := buildHierarchy([]treemapItem{
		{level: 0, name: "Root"},
		{level: 4, name: "Branch 1"},
		{level: 8, name: "Leaf 1.1", leaf: true, value: 10},
		{level: 8, name: "Leaf 1.2", leaf: true, value: 15},
		{level: 4, name: "Branch 2"},
		{level: 8, name: "Leaf 2.1", leaf: true, value: 20},
		{level: 0, name: "Other", leaf: true, value: 5},
	})
	if len(roots) != 2 || roots[1].name != "Other" {
		t.Fatalf("roots %v", roots)
	}
	branches := roots[0].children
	if len(branches) != 2 || len(branches[0].children) != 2 || len(branches[1].children) != 1 {
		t.Fatalf("branches %v", branches)
	}
	if leaf := branches[0].children[1]; leaf.name != "Leaf 1.2" || leaf.value != 15 || leaf.children != nil {
		t.Fatalf("leaf %+v", leaf)
	}
}
