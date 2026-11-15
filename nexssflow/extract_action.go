package nexssflow

import (
	"context"

	"github.com/nexssp/kernel/action"
	"github.com/nexssp/kernel/xerr"

	"github.com/nexssp/lang/extract"
)

// ExtractSourceOptions controls generic source extraction from a Flow action.
type ExtractSourceOptions struct {
	ExportedOnly   bool `json:"exported_only,omitempty"`
	StripComments  bool `json:"strip_comments,omitempty"`
	TrimIndent     bool `json:"trim_indent,omitempty"`
	TrimBlankLines bool `json:"trim_blank_lines,omitempty"`
	MaxLines       int  `json:"max_lines,omitempty"`
	MaxLinesHead   int  `json:"max_lines_head,omitempty"`
	MaxLinesTail   int  `json:"max_lines_tail,omitempty"`
	RemoveBodyFunc bool `json:"remove_body_func,omitempty"`
}

// ExtractSourceRequest supplies source text and its language to lang.extract.
// Path is a logical source filename used by language parsers; no file is read.
type ExtractSourceRequest struct {
	Path     string               `json:"path"`
	Source   string               `json:"source"`
	Language string               `json:"language"`
	Options  ExtractSourceOptions `json:"options,omitzero"`
}

// ExtractSourceResult contains declaration snippets, function count, or
// processed full content when the selected parser has no snippets to return.
type ExtractSourceResult struct {
	Snippets  []string `json:"snippets"`
	FuncCount int      `json:"func_count"`
	Content   string   `json:"content"`
}

var extractSourceAction = action.New[ExtractSourceRequest, ExtractSourceResult](
	"lang.extract",
	extractSource,
).
	Description("Extract source declarations or process source text without reading files").
	Example(ExtractSourceRequest{
		Path:     "service.go",
		Source:   "package service\n\nfunc Start() {}\n",
		Language: "go",
		Options:  ExtractSourceOptions{ExportedOnly: true},
	}).
	Build()

func extractSource(ctx context.Context, req ExtractSourceRequest) (ExtractSourceResult, error) {
	if err := ctx.Err(); err != nil {
		return ExtractSourceResult{}, xerr.Canceled("lang.extract: request canceled", err)
	}

	result, err := extract.Parse(req.Path, []byte(req.Source), req.Language, extract.Options{
		ExportedOnly:   req.Options.ExportedOnly,
		StripComments:  req.Options.StripComments,
		TrimIndent:     req.Options.TrimIndent,
		TrimBlankLines: req.Options.TrimBlankLines,
		MaxLines:       req.Options.MaxLines,
		MaxLinesHead:   req.Options.MaxLinesHead,
		MaxLinesTail:   req.Options.MaxLinesTail,
		RemoveBodyFunc: req.Options.RemoveBodyFunc,
	})
	if err != nil {
		return ExtractSourceResult{}, xerr.Validation("lang.extract: parse source", err)
	}

	return ExtractSourceResult{
		Snippets:  result.Snippets,
		FuncCount: result.FuncCount,
		Content:   result.Content,
	}, nil
}
