// Package php provides source signature extraction for php files.
package php

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

func parsePHP(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	p := sitter.NewParser()
	defer p.Close()
	p.SetLanguage(php.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, 0, err
	}
	defer tree.Close()

	root := tree.RootNode()
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		for i := range int(node.NamedChildCount()) {
			child := node.NamedChild(i)
			switch child.Type() {
			case "namespace_definition":
				body := child.ChildByFieldName("body")
				if body != nil {
					walk(body)
				}
			case tsFunctionDefinition:
				funcCount++
				body := child.ChildByFieldName("body")
				if body != nil {
					sig := strings.TrimSpace(string(src[child.StartByte():body.StartByte()]))
					snippets = append(snippets, sig+" {}")
				} else {
					snippets = append(snippets, child.Content(src))
				}
			case "class_declaration", "interface_declaration", "trait_declaration", "enum_declaration":
				classSig, count := formatPHPClass(child, src, exportedOnly)
				funcCount += count
				snippets = append(snippets, classSig)
			}
		}
	}

	walk(root)
	return snippets, funcCount, nil
}

func formatPHPClass(classNode *sitter.Node, src []byte, exportedOnly bool) (classSignature string, methodCount int) {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return classNode.Content(src), 0
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(string(src[classNode.StartByte():body.StartByte()])) + " {\n")
	count := 0

	for i := range int(body.NamedChildCount()) {
		member := body.NamedChild(i)
		switch member.Type() {
		case "method_declaration":
			if exportedOnly && !isPHPMethodPublic(member, src) {
				continue
			}
			count++
			mBody := member.ChildByFieldName("body")
			if mBody != nil {
				sb.WriteString("  " + strings.TrimSpace(string(src[member.StartByte():mBody.StartByte()])) + " {}\n")
			} else {
				sb.WriteString("  " + member.Content(src) + "\n")
			}
		case "property_declaration", "class_const_declaration":
			if exportedOnly && !isPHPMethodPublic(member, src) {
				continue
			}
			sb.WriteString("  " + member.Content(src) + "\n")
		}
	}
	sb.WriteString("}")
	return sb.String(), count
}

func isPHPMethodPublic(node *sitter.Node, src []byte) bool {
	content := node.Content(src)
	return !strings.Contains(content, "private ") && !strings.Contains(content, "protected ")
}

// ExtractSignatures parses PHP source and emits declaration signatures.
func ExtractSignatures(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	return parsePHP(src, exportedOnly)
}
