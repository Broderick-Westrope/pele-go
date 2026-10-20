package packet

import (
	"math"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type block struct {
	start, end float64
	// As Mermaid counts it: one less than the width for the pieces of a field split across rows.
	bits  float64
	label string
}

type model struct {
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
	bitsPerRow                float64
	rows                      [][]block
}

const maxRows = 10_000

func packetOption(config *jsobj.Object, key string, fallback, least, most float64) float64 {
	section, _ := config.Get("packet")
	obj, ok := section.(*jsobj.Object)
	if !ok {
		return fallback
	}
	v, _ := obj.Get(key)
	if value, isNum := v.(float64); isNum && value >= least {
		return min(value, most)
	}
	return fallback
}

func invalid(message string) {
	e := perr.New(message, perr.Semantic)
	e.Type = "packet"
	perr.Throw(e)
}

func buildPacket(a ast, config *jsobj.Object, title *string) model {
	bitsPerRow := math.Floor(packetOption(config, "bitsPerRow", 32, 1, 4096))
	var rows [][]block
	lastBit := -1.0
	var word []block
	row := 1.0

	for _, b := range a.blocks {
		start, end, bits := b.start, b.end, b.bits
		if start != nil && end != nil && *end < *start {
			invalid("Packet block " + jsnum.Format(*start) + " - " + jsnum.Format(*end) + " is invalid. End must be greater than start.")
		}
		if start == nil {
			start = new(lastBit + 1)
		}
		if *start != lastBit+1 {
			e := start
			if end != nil {
				e = end
			}
			invalid("Packet block " + jsnum.Format(*start) + " - " + jsnum.Format(*e) + " is not contiguous. It should start from " + jsnum.Format(lastBit+1) + ".")
		}
		if bits != nil && *bits == 0 {
			invalid("Packet block " + jsnum.Format(*start) + " is invalid. Cannot have a zero bit field.")
		}
		if end == nil {
			n := 1.0
			if bits != nil {
				n = *bits
			}
			end = new(*start + n - 1)
		}
		if bits == nil {
			bits = new(*end - *start + 1)
		}
		s, e, n := *start, *end, *bits
		lastBit = e

		for float64(len(word)) <= bitsPerRow+1 && len(rows) < maxRows {
			rowEnd := float64(row*bitsPerRow) - 1
			if e <= rowEnd {
				word = append(word, block{start: s, end: e, bits: n, label: b.label})
				if e == rowEnd {
					rows = append(rows, word)
					word = nil
					row++
				}
				break
			}
			word = append(word, block{start: s, end: rowEnd, bits: rowEnd - s, label: b.label})
			rows = append(rows, word)
			word = nil
			row++
			s = rowEnd + 1
			n = e - s
		}
	}
	if len(word) > 0 {
		rows = append(rows, word)
	}

	m := model{bitsPerRow: bitsPerRow, rows: rows}
	if a.title != nil && *a.title != "" {
		m.title = *a.title
	} else if title != nil {
		m.title = *title
	}
	if a.accTitle != nil && *a.accTitle != "" {
		m.accTitle = jsstr.TrimStart(*a.accTitle)
	}
	if a.accDescr != nil && *a.accDescr != "" {
		m.accDescr = common.CollapseIndentedBreaks(*a.accDescr)
	}
	return m
}
