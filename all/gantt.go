package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/gantt"
)

func init() {
	pele.Register(gantt.Diagram)
}
