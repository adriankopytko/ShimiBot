package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type WriteTool struct{}

type writeArgs struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (WriteTool) Name() string {
	return "Write"
}

func (tool WriteTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        tool.Name(),
		Description: "Write content to a file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The path to the file to write to",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			"required": []string{"file_path", "content"},
		},
	}
}

func (WriteTool) Execute(ctx ToolContext, arguments string) (any, error) {
	var args writeArgs
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

	if err := os.WriteFile(resolvedPath, []byte(args.Content), 0644); err != nil {
		return "", fmt.Errorf("error writing file: %w", err)
	}

	return map[string]any{
		"file_path": args.FilePath,
		"content":   args.Content,
	}, nil
}
