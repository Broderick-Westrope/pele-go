package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/railroad"
)

func init() {
	pele.Register(railroad.Diagram)
}
