package venn

import (
	"slices"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

// normalizeText takes one pair of quotes off an id, label or style value, as Mermaid does.
func normalizeText(text string) string {
	trimmed := jsstr.Trim(text)
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		return trimmed[1 : len(trimmed)-1]
	}
	return trimmed
}

func normalizeIds(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = normalizeText(id)
	}
	slices.SortStableFunc(out, jsstr.Compare)
	return out
}

func normalizeLabel(label string) *string {
	if label == "" {
		return nil
	}
	s := normalizeText(label)
	return &s
}

type vennDb struct {
	// title is "" where upstream's is undefined: every use treats the two alike.
	title     string
	subsets   []vennSubset
	textNodes []vennText
	styles    []vennStyle
	known     omap.Set[string]
	current   []string
	// Whether an indented `text` line belongs to the set or union before it. The lexer reads this.
	indent bool
}

func newVennDb() *vennDb {
	return &vennDb{}
}

func (db *vennDb) setDiagramTitle(title string) {
	db.title = title
}

// addSubsetData adds a set or an overlap. A set without a size gets 10, and an overlap of
// n sets 10 / n².
func (db *vennDb) addSubsetData(ids []string, label string, size *float64) {
	sets := normalizeIds(ids)
	db.current = sets
	if len(sets) == 1 {
		db.known.Add(sets[0])
	}
	n := float64(len(ids))
	value := 10 / (n * n)
	if size != nil {
		value = *size
	}
	db.subsets = append(db.subsets, vennSubset{sets: sets, size: value, label: normalizeLabel(label)})
}

func (db *vennDb) addTextData(ids []string, id string, label string) {
	db.textNodes = append(db.textNodes, vennText{sets: normalizeIds(ids), id: normalizeText(id), label: normalizeLabel(label)})
}

func (db *vennDb) addStyleData(ids []string, data [][2]string) {
	styles := &omap.Map[string, string]{}
	for _, kv := range data {
		value := kv[1]
		if value != "" {
			value = normalizeText(value)
		}
		styles.Set(kv[0], value)
	}
	db.styles = append(db.styles, vennStyle{targets: normalizeIds(ids), styles: styles})
}

func (db *vennDb) validateUnionIdentifiers(ids []string) {
	var unknown []string
	for _, id := range ids {
		if id = normalizeText(id); !db.known.Has(id) {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		throwSemantic("unknown set identifier: " + strings.Join(unknown, ", "))
	}
}

func throwSemantic(message string) {
	e := perr.New(message, perr.Semantic)
	e.Type = "venn"
	perr.Throw(e)
}
