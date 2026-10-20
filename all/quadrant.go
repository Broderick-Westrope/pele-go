package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/quadrant"
)

func init() {
	pele.Register(quadrant.Diagram)
}
