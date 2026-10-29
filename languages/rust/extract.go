package rust

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
)

func parseRust(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	p := sitter.NewParser()
	defer p.Close()
	p.SetLanguage(rust.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, 0, err
	}
	defer tree.Close()

	root := tree.RootNode()
	for i := range int(root.NamedChildCount()) {
		child := root.NamedChild(i)

		isPub := hasRustVisibility(child, src)
		if child.Type() == rustImplItem || child.Type() == tsMacroDefinition || child.Type() == rustForeignModItem {
			isPub = true
		}

		if exportedOnly && !isPub {
			continue
		}

		switch child.Type() {
		case tsFunctionItem:
			funcCount++
			body := child.ChildByFieldName("body")
			if body != nil {
				sig := strings.TrimSpace(string(src[child.StartByte():body.StartByte()]))
				snippets = append(snippets, sig+" {}")
			} else {
				snippets = append(snippets, child.Content(src))
			}

		case "struct_item", "enum_item", "union_item", "type_item", "const_item", "static_item", "use_declaration", rustForeignModItem, "macro_invocation":
			snippets = append(snippets, child.Content(src))

		case "trait_item":
			traitSig, count := formatRustTrait(child, src)
			funcCount += count
			snippets = append(snippets, traitSig)

		case rustImplItem:
			implSig, count := formatRustImpl(child, src, exportedOnly)
			funcCount += count
			if implSig != "" {
				snippets = append(snippets, implSig)
			}

		case tsMacroDefinition:
			snippets = append(snippets, formatRustMacro(child, src))

		case "mod_item":
			modSig, count := formatRustMod(child, src, exportedOnly)
			funcCount += count
			if modSig != "" {
				snippets = append(snippets, modSig)
			}
		}
	}

	return snippets, funcCount, nil
}

func hasRustVisibility(node *sitter.Node, src []byte) bool {
	for i := range int(node.NamedChildCount()) {
		child := node.NamedChild(i)
		if child.Type() == "visibility_modifier" {
			return strings.TrimSpace(child.Content(src)) != "pub(self)"
		}
	}
	return false
}

func formatRustTrait(traitNode *sitter.Node, src []byte) (traitSignature string, methodCount int) {
	body := traitNode.ChildByFieldName("body")
	if body == nil {
		return traitNode.Content(src), 0
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(string(src[traitNode.StartByte():body.StartByte()])) + " {\n")
	count := 0

	for i := range int(body.NamedChildCount()) {
		item := body.NamedChild(i)
		if item.Type() == tsFunctionItem {
			count++
			fnBody := item.ChildByFieldName("body")
			if fnBody != nil {
				sb.WriteString("  " + strings.TrimSpace(string(src[item.StartByte():fnBody.StartByte()])) + " {}\n")
			} else {
				sb.WriteString("  " + item.Content(src) + "\n")
			}
		} else {
			sb.WriteString("  " + item.Content(src) + "\n")
		}
	}
	sb.WriteString("}")
	return sb.String(), count
}

func formatRustImpl(implNode *sitter.Node, src []byte, exportedOnly bool) (implSignature string, methodCount int) {
	body := implNode.ChildByFieldName("body")
	if body == nil {
		return implNode.Content(src), 0
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(string(src[implNode.StartByte():body.StartByte()])) + " {\n")
	count := 0
	hasMembers := false
	isTraitImpl := implNode.ChildByFieldName("trait") != nil

	for i := range int(body.NamedChildCount()) {
		item := body.NamedChild(i)
		switch item.Type() {
		case tsFunctionItem:
			if exportedOnly && !isTraitImpl && !hasRustVisibility(item, src) {
				continue
			}
			count++
			hasMembers = true
			fnBody := item.ChildByFieldName("body")
			if fnBody != nil {
				sb.WriteString("  " + strings.TrimSpace(string(src[item.StartByte():fnBody.StartByte()])) + " {}\n")
			} else {
				sb.WriteString("  " + item.Content(src) + "\n")
			}
		case "type_item", "const_item", "macro_invocation":
			if exportedOnly && !isTraitImpl && !hasRustVisibility(item, src) {
				continue
			}
			hasMembers = true
			sb.WriteString("  " + item.Content(src) + "\n")
		}
	}

	if exportedOnly && !isTraitImpl && !hasMembers {
		return "", 0
	}
	sb.WriteString("}")
	return sb.String(), count
}

func formatRustMacro(node *sitter.Node, src []byte) string {
	name := node.ChildByFieldName("name")
	if name == nil {
		return node.Content(src)
	}
	prefix := strings.TrimSpace(string(src[node.StartByte():name.StartByte()]))
	if prefix == "" {
		prefix = "macro_rules!"
	}
	return fmt.Sprintf("%s %s { /* ... */ }", prefix, name.Content(src))
}

func formatRustMod(modNode *sitter.Node, src []byte, exportedOnly bool) (moduleSignature string, funcCount int) {
	body := modNode.ChildByFieldName("body")
	if body == nil {
		return modNode.Content(src), 0
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(string(src[modNode.StartByte():body.StartByte()])) + " {\n")
	count := 0

	for i := range int(body.NamedChildCount()) {
		child := body.NamedChild(i)
		if exportedOnly && !hasRustVisibility(child, src) && child.Type() != rustImplItem && child.Type() != tsMacroDefinition {
			continue
		}
		switch child.Type() {
		case tsFunctionItem:
			count++
			fnBody := child.ChildByFieldName("body")
			if fnBody != nil {
				sb.WriteString("  " + strings.TrimSpace(string(src[child.StartByte():fnBody.StartByte()])) + " {}\n")
			} else {
				sb.WriteString("  " + child.Content(src) + "\n")
			}
		case "struct_item", "enum_item", "type_item", "const_item", "static_item", "use_declaration":
			sb.WriteString("  " + child.Content(src) + "\n")
		case tsMacroDefinition:
			sb.WriteString("  " + formatRustMacro(child, src) + "\n")
		}
	}
	sb.WriteString("}")
	return sb.String(), count
}

// ExtractSignatures parses Rust source and emits declaration signatures.
func ExtractSignatures(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	return parseRust(src, exportedOnly)
}
