package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/wardley"
)

func init() {
	pele.Register(wardley.Diagram)
}
