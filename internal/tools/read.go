package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type ReadTool struct{}

type readArgs struct {
	FilePath string `json:"file_path"`
}

func (ReadTool) Name() string {
	return "Read"
}

func (tool ReadTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        tool.Name(),
		Description: "Read and return the contents of a file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The path to the file to read",
				},
			},
			"required": []string{"file_path"},
		},
	}
}

func (ReadTool) Execute(ctx ToolContext, arguments string) (any, error) {
	var args readArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}

	args.FilePath = strings.TrimSpace(args.FilePath)
	if args.FilePath == "" {
		return "", fmt.Errorf("file_path must be a non-empty string")
	}
	resolvedPath := ResolvePath(ctx, args.FilePath)
	if err := EnsurePathAllowed(ctx, resolvedPath); err != nil {
		return "", fmt.Errorf("path policy violation: %w", err)
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return string(content), nil
}
