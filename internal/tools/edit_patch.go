package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type EditPatchTool struct{}

type editPatchArgs struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

func (EditPatchTool) Name() string {
	return "EditPatch"
}

func (tool EditPatchTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        tool.Name(),
		Description: "Edit a file by replacing a target string with a new string",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact string to replace",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Replacement string",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "When true, replace all matches. Defaults to false.",
				},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		},
	}
}

func (EditPatchTool) Execute(ctx ToolContext, arguments string) (any, error) {
	var args editPatchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}

	args.FilePath = strings.TrimSpace(args.FilePath)
	if args.FilePath == "" {
		return "", fmt.Errorf("file_path must be a non-empty string")
	}
	if args.OldString == "" {
		return "", fmt.Errorf("old_string must be a non-empty string")
	}
	resolvedPath := ResolvePath(ctx, args.FilePath)
	if err := EnsurePathAllowed(ctx, resolvedPath); err != nil {
		return "", fmt.Errorf("path policy violation: %w", err)
	}

	contentBytes, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	content := string(contentBytes)
	occurrences := strings.Count(content, args.OldString)
	if occurrences == 0 {
		return "", fmt.Errorf("old_string not found in file")
	}

	newContent := content
	replacements := 1
	if args.ReplaceAll {
		newContent = strings.ReplaceAll(content, args.OldString, args.NewString)
		replacements = occurrences
	} else {
		newContent = strings.Replace(content, args.OldString, args.NewString, 1)
	}

	if err := os.WriteFile(resolvedPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("error writing file: %w", err)
	}

	return map[string]interface{}{
		"file_path":     args.FilePath,
		"replacements":  replacements,
		"replace_all":   args.ReplaceAll,
		"total_matches": occurrences,
	}, nil
}
