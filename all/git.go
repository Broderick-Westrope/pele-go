package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/git"
)

func init() {
	pele.Register(git.Diagram)
}
