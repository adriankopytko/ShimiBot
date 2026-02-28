package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type ListDirTool struct{}

type listDirArgs struct {
	Path string `json:"path"`
}

type listDirEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (ListDirTool) Name() string {
	return "ListDir"
}

func (tool ListDirTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        tool.Name(),
		Description: "List entries in a directory",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to list. Defaults to current directory when omitted.",
				},
			},
		},
	}
}

func (ListDirTool) Execute(ctx ToolContext, arguments string) (any, error) {
	var args listDirArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}

	pathValue := strings.TrimSpace(args.Path)
	if pathValue == "" {
		pathValue = "."
	}
	resolvedPath := ResolvePath(ctx, pathValue)
	if err := EnsurePathAllowed(ctx, resolvedPath); err != nil {
		return "", fmt.Errorf("path policy violation: %w", err)
	}

	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("error reading directory %q: %w", pathValue, err)
	}

	responseEntries := make([]listDirEntry, 0, len(entries))
	for _, item := range entries {
		entryType := "file"
		if item.IsDir() {
			entryType = "dir"
		}
		responseEntries = append(responseEntries, listDirEntry{
			Name: item.Name(),
			Type: entryType,
		})
	}

	return map[string]interface{}{
		"path":    pathValue,
		"entries": responseEntries,
	}, nil
}
