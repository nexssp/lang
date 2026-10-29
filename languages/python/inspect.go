package python

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nexssp/lang/contract"
)

//go:embed ast_parser.py
var astParser string

// InspectOptions configures the Python interpreter used only to run the
// standard-library AST parser. Input modules themselves are never executed.
type InspectOptions struct {
	PythonCommand string
}

type rawContract struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Fields      []rawField `json:"fields"`
}

type rawField struct {
	Name       string `json:"name"`
	Annotation string `json:"annotation"`
	Required   bool   `json:"required"`
}

// Inspect parses Python dataclasses and TypedDict declarations into the
// language-neutral contract model. If className is non-empty, only that class
// is returned; otherwise all supported top-level classes are inspected.
func Inspect(ctx context.Context, source, className string, options InspectOptions) ([]contract.Contract, error) {
	interpreter := strings.TrimSpace(options.PythonCommand)
	if interpreter == "" {
		interpreter = "python3"
	}
	command := exec.CommandContext(ctx, interpreter, "-I", "-c", astParser)
	command.Stdin = strings.NewReader(source)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("inspect Python source with %q: %s", interpreter, detail)
	}

	var parsed []rawContract
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, fmt.Errorf("decode Python AST inspection: %w", err)
	}
	result := make([]contract.Contract, 0, len(parsed))
	found := false
	for _, item := range parsed {
		if className != "" && item.Name != className {
			continue
		}
		found = true
		current := contract.Contract{Name: item.Name, Description: item.Description}
		for _, field := range item.Fields {
			valueType := mapAnnotation(field.Annotation)
			current.Fields = append(current.Fields, contract.Field{
				Name:     field.Name,
				Type:     valueType,
				Required: field.Required,
			})
		}
		if err := current.Validate(); err != nil {
			return nil, fmt.Errorf("inspect Python class %q: %w", item.Name, err)
		}
		result = append(result, current)
	}
	if className != "" && !found {
		return nil, fmt.Errorf("python contract class %q was not found", className)
	}
	return result, nil
}
