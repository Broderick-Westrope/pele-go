package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/xychart"
)

func init() {
	pele.Register(xychart.Diagram)
}
