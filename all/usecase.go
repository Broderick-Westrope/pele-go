package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/usecase"
)

func init() {
	pele.Register(usecase.Diagram)
}
