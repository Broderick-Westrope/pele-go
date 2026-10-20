package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/kanban"
)

func init() {
	pele.Register(kanban.Diagram)
}
