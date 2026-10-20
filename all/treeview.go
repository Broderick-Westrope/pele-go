package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/treeview"
)

func init() {
	pele.Register(treeview.Diagram)
}
