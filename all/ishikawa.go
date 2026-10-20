package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/ishikawa"
)

func init() {
	pele.Register(ishikawa.Diagram)
}
