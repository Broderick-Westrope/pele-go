package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/block"
)

func init() {
	pele.Register(block.Diagram)
}
