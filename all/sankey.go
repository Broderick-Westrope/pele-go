package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/sankey"
)

func init() {
	pele.Register(sankey.Diagram)
}
