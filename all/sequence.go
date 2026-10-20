package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/sequence"
)

func init() {
	pele.Register(sequence.Diagram)
}
