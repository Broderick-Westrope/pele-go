package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/treemap"
)

func init() {
	pele.Register(treemap.Diagram)
}
