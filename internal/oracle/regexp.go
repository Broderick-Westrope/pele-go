package oracle

import "strings"

// stickyBoundaries rewrites the sticky regexes that end in \b after a word character so
// goja reads them as V8 does. goja runs a sticky regex on regexp2, whose \b counts any
// Unicode letter as a word character where JS counts only [A-Za-z0-9_]: "LR" followed by
// "é" would not be a direction. After a word character, \b is a lookahead for a non-word one.
var stickyBoundaries = strings.NewReplacer(
	`|TD|BR)\b/y`, `|TD|BR)(?![A-Za-z0-9_])/y`,
	`/\s*v\b/y`, `/\s*v(?![A-Za-z0-9_])/y`,
)
