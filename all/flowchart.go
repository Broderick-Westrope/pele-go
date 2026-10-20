package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/flowchart"
)

func init() {
	pele.Register(flowchart.Diagram)
}
