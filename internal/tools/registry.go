package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type Tool interface {
	Name() string
	Definition() llm.ToolDefinition
	Execute(ctx ToolContext, arguments string) (any, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(toolList ...Tool) *Registry {
	byName := make(map[string]Tool, len(toolList))
	for _, tool := range toolList {
		byName[tool.Name()] = tool
	}
	return &Registry{tools: byName}
}

func DefaultRegistry() *Registry {
	return NewRegistry(
		BashTool{},
		EditPatchTool{},
		FetchWebPageTool{},
		WebSearchOllamaTool{},
		ReadTool{},
		WriteTool{},
		ListDirTool{},
	)
}

func (registry *Registry) Definitions() []llm.ToolDefinition {
	names := make([]string, 0, len(registry.tools))
	for name := range registry.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]llm.ToolDefinition, 0, len(names))
	for _, name := range names {
		defs = append(defs, registry.tools[name].Definition())
	}
	return defs
}

func (registry *Registry) Execute(toolCall llm.ToolCall, toolContext ToolContext) (string, bool) {
	tool, ok := registry.tools[toolCall.Name]
	if !ok {
		return "", false
	}

	meta := map[string]any{"tool": tool.Name()}
	if strings.TrimSpace(toolContext.CWD) != "" {
		meta["cwd"] = toolContext.CWD
	}
	if strings.TrimSpace(toolContext.CorrelationID) != "" {
		meta["correlation_id"] = toolContext.CorrelationID
	}
	if strings.TrimSpace(toolContext.AllowedRoot) != "" {
		meta["allowed_root"] = toolContext.AllowedRoot
	}
	if toolContext.Timeout > 0 {
		meta["timeout_ms"] = toolContext.Timeout.Milliseconds()
	}

	if strings.TrimSpace(toolContext.CWD) == "" || strings.TrimSpace(toolContext.AllowedRoot) == "" {
		return ErrorEnvelope("invalid tool context: cwd and allowed_root are required", meta), true
	}

	if toolContext.Context != nil {
		if err := toolContext.Context.Err(); err != nil {
			if err == context.Canceled {
				return ErrorEnvelope("tool execution cancelled", meta), true
			}
			return ErrorEnvelope(fmt.Sprintf("tool execution context error: %v", err), meta), true
		}
	}

	normalizedArguments, valid := NormalizeJSONArguments(toolCall.Arguments)
	if !valid {
		return ErrorEnvelope(fmt.Sprintf("%s: invalid JSON arguments", tool.Name()), meta), true
	}

	result, err := tool.Execute(toolContext, normalizedArguments)
	if err != nil {
		return ErrorEnvelope(fmt.Sprintf("%s: %v", tool.Name(), err), meta), true
	}

	return SuccessEnvelope(result, meta), true
}
