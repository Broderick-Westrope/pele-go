package mindmap

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

func parseMindmap(src string, db common.OutlineDB) {
	common.ParseOutline(jsstr.UTF16(src), db, false)
}
