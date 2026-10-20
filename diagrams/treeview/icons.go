package treeview

import (
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// builtinPack is Mermaid's name for its built-in pack. Only `file` and `folder` are built in;
// every other icon name is offered to the host's icon resolver.
const builtinPack = "mermaid-treeview"

// glyphs are outlines on a 24 by 24 grid.
var glyphs = map[string]string{
	"folder": "M3 6.5A1.5 1.5 0 0 1 4.5 5h4.6l2.4 2.5h8A1.5 1.5 0 0 1 21 9v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18z",
	"file":   "M6.5 3h7L19 8.5v11a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 5 19.5v-15A1.5 1.5 0 0 1 6.5 3zM13.5 3v5.5H19",
}

type iconSettings struct {
	showIcons       bool
	defaultIconPack string
	// nil is upstream's undefined.
	filenameIcons, extensionIcons *jsobj.Object
}

// own returns map[key] when the map has it as its own and it is a string.
func own(m *jsobj.Object, key string) (string, bool) {
	v, ok := m.Get(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// detectIcon is a file's icon from the configured maps. A file name wins over an extension,
// and extensions match in lower case with or without the leading dot.
func detectIcon(name string, config iconSettings) (string, bool) {
	if byName, _ := own(config.filenameIcons, name); byName != "" {
		return byName, true
	}
	if dot := strings.LastIndexByte(name, '.'); dot > 0 {
		ext := toLowerCase(name[dot:])
		if icon, ok := own(config.extensionIcons, ext); ok {
			return icon, true
		}
		return own(config.extensionIcons, ext[1:])
	}
	return "", false
}

func qualifyIcon(icon, defaultIconPack string) string {
	if strings.Contains(icon, ":") {
		return icon
	}
	if _, ok := glyphs[icon]; ok || defaultIconPack == "" {
		return builtinPack + ":" + icon
	}
	return defaultIconPack + ":" + icon
}

// getNodeIcon is the qualified icon name for a node, or "" when it shows none. An `icon()`
// annotation always wins; otherwise icons appear only with `showIcons`.
func getNodeIcon(node *treeNode, config iconSettings) string {
	if node.icon == "none" {
		return ""
	}
	if node.icon != "" {
		return qualifyIcon(node.icon, config.defaultIconPack)
	}
	if !config.showIcons {
		return ""
	}
	if node.nodeType == nodeFile {
		detected, _ := detectIcon(node.name, config)
		if detected == "none" {
			return ""
		}
		if detected != "" {
			return qualifyIcon(detected, config.defaultIconPack)
		}
	}
	if node.nodeType == nodeDirectory {
		return builtinPack + ":folder"
	}
	return builtinPack + ":file"
}

func iconConfig(config *jsobj.Object) iconSettings {
	asMap := func(v any) *jsobj.Object {
		o, _ := v.(*jsobj.Object)
		return o
	}
	section, _ := config.Get("treeView")
	c := asMap(section)
	showIcons, _ := c.Get("showIcons")
	pack, _ := c.Get("defaultIconPack")
	packName, _ := pack.(string)
	filenameIcons, _ := c.Get("filenameIcons")
	extensionIcons, _ := c.Get("extensionIcons")
	return iconSettings{
		showIcons:       jsobj.Truthy(showIcons),
		defaultIconPack: packName,
		filenameIcons:   asMap(filenameIcons),
		extensionIcons:  asMap(extensionIcons),
	}
}

// toLowerCase is String.prototype.toLowerCase: Unicode's default lower-casing, with İ's
// two-character lower case and Σ's final form, which Go's simple mappings leave out.
func toLowerCase(s string) string {
	units := jsstr.UTF16(s)
	var runes []rune
	for i := 0; i < len(units); {
		r := rune(units[i])
		n := 1
		if utf16.IsSurrogate(r) && i+1 < len(units) {
			if pair := utf16.DecodeRune(r, rune(units[i+1])); pair != unicode.ReplacementChar {
				r, n = pair, 2
			}
		}
		runes = append(runes, r)
		i += n
	}
	var out []uint16
	for i, r := range runes {
		switch {
		case r == 0x130:
			out = append(out, 'i', 0x307)
		case r == 0x3a3:
			if finalSigma(runes, i) {
				out = append(out, 0x3c2)
			} else {
				out = append(out, 0x3c3)
			}
		case utf16.IsSurrogate(r):
			out = append(out, uint16(r))
		default:
			out = utf16.AppendRune(out, unicode.ToLower(r))
		}
	}
	return jsstr.FromUTF16(out)
}

// finalSigma is Unicode's Final_Sigma condition: a cased letter before, skipping
// case-ignorable characters, and none after.
func finalSigma(runes []rune, at int) bool {
	before := false
	for i := at - 1; i >= 0; i-- {
		if !caseIgnorable(runes[i]) {
			before = cased(runes[i])
			break
		}
	}
	if !before {
		return false
	}
	for i := at + 1; i < len(runes); i++ {
		if !caseIgnorable(runes[i]) {
			return !cased(runes[i])
		}
	}
	return true
}

func cased(r rune) bool {
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r) ||
		unicode.Is(unicode.Other_Lowercase, r) || unicode.Is(unicode.Other_Uppercase, r)
}

func caseIgnorable(r rune) bool {
	switch r {
	case '\'', '.', ':', '^', '`', 0xa8, 0xad, 0xaf, 0xb4, 0xb7, 0xb8, 0x2018, 0x2019, 0x2024, 0x2027:
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk)
}
