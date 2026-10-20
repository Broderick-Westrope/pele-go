package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/c4"
)

func init() {
	pele.Register(c4.Diagram)
}
