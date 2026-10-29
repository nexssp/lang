package javascript

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

func parseTSJS(src []byte, filename string, exportedOnly bool) (snippets []string, funcCount int, err error) {
	p := sitter.NewParser()
	defer p.Close()

	if strings.HasSuffix(filename, ".tsx") || strings.HasSuffix(filename, ".jsx") {
		p.SetLanguage(tsx.GetLanguage())
	} else {
		p.SetLanguage(typescript.GetLanguage())
	}

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, 0, err
	}
	defer tree.Close()

	root := tree.RootNode()
	for i := range int(root.NamedChildCount()) {
		child := root.NamedChild(i)
		target, isExport := unwrapTSExport(child)
		if exportedOnly && !isExport {
			continue
		}
		startByte := tsDeclarationStart(child, target, isExport)
		declSnippets, declFuncs := emitTSDeclaration(target, src, startByte)
		snippets = append(snippets, declSnippets...)
		funcCount += declFuncs
	}

	return snippets, funcCount, nil
}

// unwrapTSExport peels an export_statement wrapper, returning the
// declaration node it contains and whether the wrapper was present.
func unwrapTSExport(child *sitter.Node) (*sitter.Node, bool) {
	if child.Type() != "export_statement" {
		return child, false
	}
	if child.NamedChildCount() > 0 {
		return child.NamedChild(0), true
	}
	return child, true
}

// tsDeclarationStart returns the byte offset where the declaration
// starts: the export keyword when wrapped, otherwise the declaration.
func tsDeclarationStart(child, target *sitter.Node, isExport bool) uint32 {
	if isExport {
		return child.StartByte()
	}
	return target.StartByte()
}

func emitTSDeclaration(target *sitter.Node, src []byte, startByte uint32) (snippets []string, funcCount int) {
	switch target.Type() {
	case "function_declaration", "generator_function_declaration":
		return []string{emitTSFunction(target, src, startByte)}, 1

	case "interface_declaration", "type_alias_declaration", "enum_declaration", "ambient_declaration":
		return []string{strings.TrimSpace(string(src[startByte:target.EndByte()]))}, 0

	case "class_declaration", "abstract_class_declaration":
		classStr, methodsCount := formatClassNode(target, src, startByte)
		return []string{classStr}, methodsCount

	case "lexical_declaration", "variable_declaration":
		return emitTSVariableDeclaration(target, src, startByte)

	default:
		return nil, 0
	}
}

func emitTSFunction(target *sitter.Node, src []byte, startByte uint32) string {
	body := target.ChildByFieldName("body")
	if body != nil {
		sig := strings.TrimSpace(string(src[startByte:body.StartByte()]))
		return sig + " {}"
	}
	return strings.TrimSpace(string(src[startByte:target.EndByte()]))
}

// emitTSVariableDeclaration walks the declarators of a `const`/`let`/
// `var` statement. Arrow functions and function expressions collapse
// to a stripped signature; everything else is emitted verbatim.
func emitTSVariableDeclaration(target *sitter.Node, src []byte, startByte uint32) (snippets []string, funcCount int) {
	for v := range int(target.NamedChildCount()) {
		declarator := target.NamedChild(v)
		val := declarator.ChildByFieldName("value")
		if val != nil && (val.Type() == "arrow_function" || val.Type() == "function_expression") {
			funcCount++
			body := val.ChildByFieldName("body")
			if body != nil && body.Type() == "statement_block" {
				sig := strings.TrimSpace(string(src[startByte:body.StartByte()]))
				snippets = append(snippets, sig+" {}")
				continue
			}
		}
		snippets = append(snippets, strings.TrimSpace(string(src[startByte:target.EndByte()])))
	}
	return snippets, funcCount
}

func formatClassNode(classNode *sitter.Node, src []byte, startByte uint32) (classSignature string, methodCount int) {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return classNode.Content(src), 0
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(string(src[startByte:body.StartByte()])) + " {\n")
	methodsCount := 0

	for i := range int(body.NamedChildCount()) {
		member := body.NamedChild(i)
		switch member.Type() {
		case "method_definition", "abstract_method_signature":
			methodsCount++
			mBody := member.ChildByFieldName("body")
			if mBody != nil {
				sb.WriteString("  " + strings.TrimSpace(string(src[member.StartByte():mBody.StartByte()])) + " {}\n")
			} else {
				sb.WriteString("  " + member.Content(src) + "\n")
			}
		case "public_field_definition", "field_definition", "property_signature", "abstract_property_signature":
			sb.WriteString("  " + member.Content(src) + "\n")
		}
	}
	sb.WriteString("}")
	return sb.String(), methodsCount
}

// ExtractSignatures parses JavaScript/TypeScript source. filename selects JS/TSX grammar where relevant.
func ExtractSignatures(src []byte, filename string, exportedOnly bool) (snippets []string, funcCount int, err error) {
	return parseTSJS(src, filename, exportedOnly)
}
