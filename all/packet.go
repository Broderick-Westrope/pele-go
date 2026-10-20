package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/packet"
)

func init() {
	pele.Register(packet.Diagram)
}
