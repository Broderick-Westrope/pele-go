package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/cynefin"
)

func init() {
	pele.Register(cynefin.Diagram)
}
