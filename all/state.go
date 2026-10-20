package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/state"
)

func init() {
	pele.Register(state.Diagram)
}
