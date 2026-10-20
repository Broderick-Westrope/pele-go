package common

import "github.com/Broderick-Westrope/pele-go/internal/core"

// maxWidth is options.maxWidth when it is set and positive.
func maxWidth(options core.Options) (float64, bool) {
	if options.MaxWidth == nil || !(*options.MaxWidth > 0) {
		return 0, false
	}
	return *options.MaxWidth, true
}

// FitWidth draws a chart whose plot can be any width so that the whole diagram fits the
// width the host has. What surrounds the plot (labels, a legend) is only known once it is
// drawn, so the plot gives up the overflow and is drawn again. narrow tells the chart that
// it did not fit at its own width, for a chart that then arranges itself differently.
func FitWidth(options core.Options, natural, least float64, draw func(plot float64, narrow bool) core.Rendered) core.Rendered {
	result := draw(natural, false)
	available, ok := maxWidth(options)
	if !ok || result.Width <= available {
		return result
	}
	floor := min(least, natural)
	plot := natural
	result = draw(plot, true)
	for round := 0; round < 3 && result.Width > available && plot > floor; round++ {
		plot = max(floor, plot-(result.Width-available))
		result = draw(plot, true)
	}
	return result
}

// TitleRoom is the width a title may take before it wraps.
func TitleRoom(options core.Options) float64 {
	available, ok := maxWidth(options)
	if !ok {
		return 4000
	}
	padding := 8.0
	if options.Padding != nil {
		padding = *options.Padding
	}
	return max(80, available-float64(2*padding))
}

// CapWidth is the width of a chart that is drawn to a set width: its own, or the host's
// when that is smaller. around is what the chart adds outside that width.
func CapWidth(options core.Options, natural, least, around float64) float64 {
	available, ok := maxWidth(options)
	if !ok {
		return natural
	}
	return max(min(least, natural), min(natural, available-around))
}

const breakpoint = 640

// TurnToFit draws a diagram that can run across or down: across, and down instead when the
// host allows it, the width it has is under the breakpoint, across does not fit, and down is
// narrower.
func TurnToFit(options core.Options, draw func(down bool) core.Rendered) core.Rendered {
	across := draw(false)
	available, ok := maxWidth(options)
	if options.AutoDirection != nil && !*options.AutoDirection || !ok || across.Width <= available {
		return across
	}
	limit := float64(breakpoint)
	if b := options.DirectionBreakpoint; b != nil && *b > 0 {
		limit = *b
	}
	if available >= limit {
		return across
	}
	down := draw(true)
	if down.Width < across.Width {
		return down
	}
	return across
}

// RunsAcross reports whether a direction runs across. "" is upstream's undefined.
func RunsAcross(direction string) bool {
	return direction == "LR" || direction == "RL"
}

// Tighten draws a diagram with room to spare, and tightly instead when that does not fit the
// width the host has and tight is narrower.
func Tighten(options core.Options, draw func(tight bool) core.Rendered) core.Rendered {
	loose := draw(false)
	available, ok := maxWidth(options)
	if !ok || loose.Width <= available {
		return loose
	}
	tight := draw(true)
	if tight.Width < loose.Width {
		return tight
	}
	return loose
}
