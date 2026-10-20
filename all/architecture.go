package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/architecture"
)

func init() {
	pele.Register(architecture.Diagram)
}
