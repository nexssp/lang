package python

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// parsePython extracts function and class signatures.
// When exportedOnly=true, only top-level functions/classes not starting with '_' are kept.
//
//nolint:gocyclo // top-level walk over four node kinds; a helper would duplicate the export filter.
func parsePython(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	p := sitter.NewParser()
	defer p.Close()
	p.SetLanguage(python.GetLanguage())

	tree, err := p.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, 0, err
	}
	defer tree.Close()

	root := tree.RootNode()
	for i := range int(root.NamedChildCount()) {
		child := root.NamedChild(i)

		switch child.Type() {
		case tsFunctionDefinition:
			nameNode := child.ChildByFieldName("name")
			if nameNode == nil {
				continue
			}
			name := nameNode.Content(src)
			if exportedOnly && strings.HasPrefix(name, "_") {
				continue
			}
			funcCount++
			body := child.ChildByFieldName("body")
			if body != nil {
				sig := strings.TrimSpace(string(src[child.StartByte():body.StartByte()]))
				snippets = append(snippets, sig+" {}")
			} else {
				snippets = append(snippets, child.Content(src))
			}

		case "class_definition":
			nameNode := child.ChildByFieldName("name")
			if nameNode == nil {
				continue
			}
			name := nameNode.Content(src)
			if exportedOnly && strings.HasPrefix(name, "_") {
				continue
			}
			sig, count := formatPythonClass(child, src, exportedOnly)
			funcCount += count
			snippets = append(snippets, sig)

		case "decorated_definition":
			defNode := child.ChildByFieldName("definition")
			if defNode == nil {
				for j := range int(child.NamedChildCount()) {
					c := child.NamedChild(j)
					if c.Type() == tsFunctionDefinition || c.Type() == "class_definition" {
						defNode = c
						break
					}
				}
			}
			if defNode == nil {
				continue
			}

			nameNode := defNode.ChildByFieldName("name")
			if nameNode == nil {
				continue
			}
			name := nameNode.Content(src)
			if exportedOnly && strings.HasPrefix(name, "_") {
				continue
			}

			if defNode.Type() == tsFunctionDefinition {
				funcCount++
				body := defNode.ChildByFieldName("body")
				if body != nil {
					sig := strings.TrimSpace(string(src[child.StartByte():body.StartByte()]))
					snippets = append(snippets, sig+" {}")
				} else {
					snippets = append(snippets, child.Content(src))
				}
			} else if defNode.Type() == "class_definition" {
				sig, count := formatPythonClass(defNode, src, exportedOnly)
				decPrefix := strings.TrimSpace(string(src[child.StartByte():defNode.StartByte()]))
				if decPrefix != "" {
					sig = decPrefix + "\n" + sig
				}
				funcCount += count
				snippets = append(snippets, sig)
			}
		}
	}

	return snippets, funcCount, nil
}

// formatPythonClass strips method bodies and optionally filters private methods.
func formatPythonClass(classNode *sitter.Node, src []byte, exportedOnly bool) (classSignature string, methodCount int) {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return classNode.Content(src), 0
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(string(src[classNode.StartByte():body.StartByte()])) + ":\n")
	count := 0

	for i := range int(body.NamedChildCount()) {
		member := body.NamedChild(i)
		if member.Type() != tsFunctionDefinition {
			continue
		}
		nameNode := member.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		name := nameNode.Content(src)
		if exportedOnly && strings.HasPrefix(name, "_") {
			continue
		}
		count++
		mBody := member.ChildByFieldName("body")
		if mBody != nil {
			sig := strings.TrimSpace(string(src[member.StartByte():mBody.StartByte()]))
			sb.WriteString("    " + sig + " {}\n")
		} else {
			sb.WriteString("    " + member.Content(src) + "\n")
		}
	}
	return sb.String(), count
}

// ExtractSignatures parses Python source and emits function and class signatures.
func ExtractSignatures(src []byte, exportedOnly bool) (snippets []string, funcCount int, err error) {
	return parsePython(src, exportedOnly)
}
