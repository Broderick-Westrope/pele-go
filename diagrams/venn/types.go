package venn

import "github.com/Broderick-Westrope/pele-go/internal/omap"

// A set (one id) or an overlap of sets (two or more ids, sorted), with its relative area.
type vennSubset struct {
	sets []string
	size float64
	// label is nil where upstream's is undefined; "" is a label of its own.
	label *string
}

// A line of text shown inside the region of the given sets.
type vennText struct {
	sets  []string
	id    string
	label *string
}

// Targets are the sorted ids of a set or overlap, or the id of a text.
type vennStyle struct {
	targets []string
	styles  *omap.Map[string, string]
}
