package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/swimlane"
)

func init() {
	pele.Register(swimlane.Diagram)
}
