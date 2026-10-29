// Package contract defines language-neutral data contracts that can be
// inspected from source code and rendered into host formats such as NFlow.
package contract

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nexssp/kernel/xerr"
)

// TypeKind is a language-independent value category.
type TypeKind string

const (
	String  TypeKind = "string"
	Integer TypeKind = "integer"
	Number  TypeKind = "number"
	Boolean TypeKind = "boolean"
	Object  TypeKind = "object"
	Array   TypeKind = "array"
	Any     TypeKind = "any"
)

// ValueType describes a value without carrying source-language syntax.
// Element is used only when Kind is Array; a nil element means any.
type ValueType struct {
	Kind    TypeKind   `json:"kind"`
	Element *ValueType `json:"element,omitempty"`
}

// Field describes one named value in a contract. JSONName is optional and
// defaults to Name; Required describes whether the field must be present.
type Field struct {
	Name     string    `json:"name"`
	JSONName string    `json:"json_name,omitempty"`
	Type     ValueType `json:"type"`
	Required bool      `json:"required"`
}

// Contract is a named, language-neutral object shape.
type Contract struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Fields      []Field `json:"fields"`
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Validate checks names, field uniqueness, and recursive value types.
func (c Contract) Validate() error {
	if !identifier.MatchString(c.Name) {
		return fmt.Errorf("contract name %q is not a valid NFlow schema identifier", c.Name)
	}
	if len(c.Fields) == 0 {
		return fmt.Errorf("contract %q has no fields", c.Name)
	}
	seenNames := make(map[string]struct{}, len(c.Fields))
	seenJSONNames := make(map[string]struct{}, len(c.Fields))
	for i, field := range c.Fields {
		if !identifier.MatchString(field.Name) {
			return fmt.Errorf("contract %q field %d name %q is not a valid identifier", c.Name, i+1, field.Name)
		}
		if strings.ContainsAny(field.JSONName, "`\r\n") {
			return fmt.Errorf("contract %q field %q has an unsupported JSON name", c.Name, field.Name)
		}
		if _, exists := seenNames[field.Name]; exists {
			return fmt.Errorf("contract %q repeats field %q", c.Name, field.Name)
		}
		seenNames[field.Name] = struct{}{}
		jsonName := field.JSONName
		if jsonName == "" {
			jsonName = field.Name
		}
		if _, exists := seenJSONNames[jsonName]; exists {
			return fmt.Errorf("contract %q repeats JSON field name %q", c.Name, jsonName)
		}
		seenJSONNames[jsonName] = struct{}{}
		if err := validateType(field.Type, 0); err != nil {
			return fmt.Errorf("contract %q field %q: %w", c.Name, field.Name, err)
		}
	}
	return nil
}

// ValidateAll checks every contract and rejects duplicate schema names.
func ValidateAll(contracts []Contract) error {
	seen := make(map[string]struct{}, len(contracts))
	for _, item := range contracts {
		if err := item.Validate(); err != nil {
			return err
		}
		if _, exists := seen[item.Name]; exists {
			return fmt.Errorf("schema name %q is repeated", item.Name)
		}
		seen[item.Name] = struct{}{}
	}
	return nil
}

func validateType(value ValueType, depth int) error {
	if depth > 16 {
		return xerr.Validation("type nesting exceeds 16 levels")
	}
	switch value.Kind {
	case String, Integer, Number, Boolean, Object, Any:
		if value.Element != nil {
			return fmt.Errorf("type %q cannot have an element type", value.Kind)
		}
	case Array:
		if value.Element != nil {
			return validateType(*value.Element, depth+1)
		}
	default:
		return fmt.Errorf("unsupported type kind %q", value.Kind)
	}
	return nil
}
