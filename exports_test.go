package pele_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiagramPackagesExportOnlyDiagram keeps the diagrams packages' surface to the Diagram
// variable Register takes; anything else exported would leak internals into the public API.
func TestDiagramPackagesExportOnlyDiagram(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("diagrams", "*", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		var names []*ast.Ident
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					names = append(names, decl.Name)
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name)
					case *ast.ValueSpec:
						names = append(names, spec.Names...)
					}
				}
			}
		}
		for _, name := range names {
			if name.IsExported() && name.Name != "Diagram" {
				t.Errorf("%s exports %s", fset.Position(name.Pos()), name.Name)
			}
		}
	}
}
