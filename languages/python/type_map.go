package python

import (
	"strings"

	"github.com/nexssp/lang/contract"
)

func mapAnnotation(annotation string) contract.ValueType {
	annotation = strings.TrimSpace(annotation)
	annotation = strings.Trim(annotation, "\"'")
	if annotation == "" {
		return contract.ValueType{Kind: contract.Any}
	}

	union := splitTopLevel(annotation, '|')
	if len(union) > 1 {
		return mapUnionAnnotation(union)
	}

	base, args, generic := genericType(annotation)
	name := shortTypeName(base)
	if generic {
		return mapGenericType(name, args)
	}

	return mapSimpleType(name)
}

func mapUnionAnnotation(members []string) contract.ValueType {
	remaining := make([]string, 0, len(members))
	for _, member := range members {
		member = strings.TrimSpace(member)
		if member != "None" {
			remaining = append(remaining, member)
		}
	}
	if len(remaining) == 1 {
		return mapAnnotation(remaining[0])
	}
	return contract.ValueType{Kind: contract.Any}
}

func mapGenericType(name string, args []string) contract.ValueType {
	switch name {
	case "Optional", "Required", "NotRequired", "Annotated":
		return mapFirstGenericType(args)
	case "Union":
		return mapUnionAnnotation(args)
	case "list", "List", "Sequence", "Iterable", "Iterator", "set", "Set", "frozenset":
		return mapCollectionType(args)
	case "tuple", "Tuple":
		return mapTupleType(args)
	case "dict", "Dict", "Mapping", "MutableMapping":
		return contract.ValueType{Kind: contract.Object}
	case "Literal":
		return contract.ValueType{Kind: contract.Any}
	default:
		return contract.ValueType{Kind: contract.Object}
	}
}

func mapFirstGenericType(args []string) contract.ValueType {
	if len(args) > 0 {
		return mapAnnotation(args[0])
	}
	return contract.ValueType{Kind: contract.Any}
}

func mapCollectionType(args []string) contract.ValueType {
	element := contract.ValueType{Kind: contract.Any}
	if len(args) > 0 && strings.TrimSpace(args[0]) != "..." {
		element = mapAnnotation(args[0])
	}
	return contract.ValueType{Kind: contract.Array, Element: &element}
}

func mapTupleType(args []string) contract.ValueType {
	element := contract.ValueType{Kind: contract.Any}
	if len(args) == 1 && strings.TrimSpace(args[0]) != "..." {
		element = mapAnnotation(args[0])
	} else if len(args) == 2 && strings.TrimSpace(args[1]) == "..." {
		element = mapAnnotation(args[0])
	}
	// Fixed-length tuples with multiple positions are represented as arrays of any.
	return contract.ValueType{Kind: contract.Array, Element: &element}
}

func mapSimpleType(name string) contract.ValueType {
	switch name {
	case "str", "bytes", "bytearray":
		return contract.ValueType{Kind: contract.String}
	case "int":
		return contract.ValueType{Kind: contract.Integer}
	case "float":
		return contract.ValueType{Kind: contract.Number}
	case "bool":
		return contract.ValueType{Kind: contract.Boolean}
	case "Any", "object", "None", "NoneType":
		return contract.ValueType{Kind: contract.Any}
	case "list", "List", "Sequence", "Iterable", "Iterator", "set", "Set", "frozenset", "tuple", "Tuple":
		element := contract.ValueType{Kind: contract.Any}
		return contract.ValueType{Kind: contract.Array, Element: &element}
	case "dict", "Dict", "Mapping", "MutableMapping":
		return contract.ValueType{Kind: contract.Object}
	default:
		// Unknown Python classes are object-shaped; custom validation or
		// nested contract extraction can refine them in a later stage.
		return contract.ValueType{Kind: contract.Object}
	}
}

func genericType(annotation string) (base string, args []string, ok bool) {
	open := strings.IndexByte(annotation, '[')
	if open < 0 || !strings.HasSuffix(strings.TrimSpace(annotation), "]") {
		return annotation, nil, false
	}
	depth := 0
	closingBracket := -1
	for i := open; i < len(annotation); i++ {
		switch annotation[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				closingBracket = i
			}
		}
		if depth < 0 {
			return annotation, nil, false
		}
	}
	if closingBracket != len(strings.TrimSpace(annotation))-1 || depth != 0 {
		return annotation, nil, false
	}
	return strings.TrimSpace(annotation[:open]), splitTopLevel(annotation[open+1:closingBracket], ','), true
}

func splitTopLevel(source string, separator byte) []string {
	var parts []string
	start := 0
	var round, square, curly int
	var quote byte
	escaped := false
	for i := range len(source) {
		ch := source[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		default:
			if ch == separator && round == 0 && square == 0 && curly == 0 {
				parts = append(parts, strings.TrimSpace(source[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(source[start:]))
	return parts
}

func shortTypeName(name string) string {
	name = strings.TrimSpace(name)
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	return name
}
