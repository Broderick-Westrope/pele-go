package treeview

import (
	"strings"

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
		ext := jsstr.ToLowerCase(name[dot:])
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
