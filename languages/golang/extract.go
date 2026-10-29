package golang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

//nolint:gocyclo // AST decl walk; each case is a distinct go/ast node kind with its own filtering.
func parseGo(path string, src []byte, exportedOnly bool, stripComments ...bool) (snippets []string, funcCount int, err error) {
	shouldStripComments := len(stripComments) > 0 && stripComments[0]
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, 0, fmt.Errorf("parse go file %s: %w", path, err)
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			// Ignore imports in signature maps to save tokens
			if d.Tok == token.IMPORT {
				continue
			}

			// Clear doc comments before AST node formatting so they don't produce blank lines
			if shouldStripComments {
				d.Doc = nil
			}

			if exportedOnly {
				var filteredSpecs []ast.Spec
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if shouldStripComments {
							s.Doc = nil
							s.Comment = nil
						}
						if ast.IsExported(s.Name.Name) {
							filteredSpecs = append(filteredSpecs, s)
						}
					case *ast.ValueSpec:
						if shouldStripComments {
							s.Doc = nil
							s.Comment = nil
						}
						var expNames []*ast.Ident
						for _, id := range s.Names {
							if ast.IsExported(id.Name) {
								expNames = append(expNames, id)
							}
						}
						if len(expNames) > 0 {
							s.Names = expNames
							filteredSpecs = append(filteredSpecs, s)
						}
					}
				}
				if len(filteredSpecs) == 0 {
					continue
				}
				d.Specs = filteredSpecs
			} else if shouldStripComments {
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						s.Doc = nil
						s.Comment = nil
					case *ast.ValueSpec:
						s.Doc = nil
						s.Comment = nil
					}
				}
			}

			var buf bytes.Buffer
			if err := format.Node(&buf, fset, d); err == nil {
				snippet := strings.TrimSpace(buf.String())
				if snippet != "" {
					snippets = append(snippets, snippet)
				}
			}

		case *ast.FuncDecl:
			isExported := ast.IsExported(d.Name.Name)
			// Treat main() in package main as the visible public entry point
			if file.Name.Name == "main" && d.Name.Name == "main" {
				isExported = true
			}
			if exportedOnly && !isExported {
				continue
			}
			funcCount++

			if shouldStripComments {
				d.Doc = nil
			}

			origBody := d.Body
			d.Body = nil
			var buf bytes.Buffer
			if err := format.Node(&buf, fset, d); err == nil {
				snippet := strings.TrimSpace(buf.String()) + " {}"
				snippets = append(snippets, snippet)
			}
			d.Body = origBody
		}
	}

	return snippets, funcCount, nil
}

// ExtractSignatures parses Go source and emits declaration signatures.
func ExtractSignatures(path string, src []byte, exportedOnly, stripComments bool) (snippets []string, funcCount int, err error) {
	return parseGo(path, src, exportedOnly, stripComments)
}
