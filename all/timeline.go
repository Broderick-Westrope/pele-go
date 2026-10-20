package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/timeline"
)

func init() {
	pele.Register(timeline.Diagram)
}
