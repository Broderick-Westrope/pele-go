// Package all registers every diagram type the port has. Import it for its side effect.
package all

import (
	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/diagrams/info"
)

func init() {
	pele.Register(info.Diagram)
}
