package kanban

import "github.com/Broderick-Westrope/pele-go/internal/diagrams/common"

func parseKanban(src common.Src, db common.OutlineDB) {
	common.ParseOutline(src, db, true)
}
