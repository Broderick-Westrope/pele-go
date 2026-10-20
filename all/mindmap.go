package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/mindmap"
)

func init() {
	pele.Register(mindmap.Diagram)
}
