package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/pie"
)

func init() {
	pele.Register(pie.Diagram)
}
