package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/er"
)

func init() {
	pele.Register(er.Diagram)
}
