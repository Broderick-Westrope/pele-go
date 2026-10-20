// Package flowchart is the flowchart diagram type. The implementation is in
// internal/diagrams/flowchart, which the types built on flowcharts share.
package flowchart

import "github.com/Broderick-Westrope/pele-go/internal/diagrams/flowchart"

// Diagram is the flowchart diagram type. Register it with pele.Register.
var Diagram = flowchart.Diagram
