package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/eventmodeling"
)

func init() {
	pele.Register(eventmodeling.Diagram)
}
