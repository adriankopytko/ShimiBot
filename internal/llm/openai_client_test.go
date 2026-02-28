package llm

import (
	"testing"

	"github.com/openai/openai-go/v3"
)

func TestToOpenAIMessages_AssistantToolCalls(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "system prompt"},
		{Role: RoleUser, Content: "user prompt"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_1", Name: "ListDir", Arguments: "{}"}}},
		{Role: RoleTool, ToolCallID: "call_1", Content: `{"entries":[]}`},
	}

	params, err := toOpenAIMessages(messages)
	if err != nil {
		t.Fatalf("toOpenAIMessages returned error: %v", err)
	}
	if len(params) != 4 {
		t.Fatalf("expected 4 params, got %d", len(params))
	}
	if params[2].OfAssistant == nil || len(params[2].OfAssistant.ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call conversion")
	}
	if params[2].OfAssistant.ToolCalls[0].OfFunction == nil {
		t.Fatalf("expected function tool call variant")
	}
	if params[2].OfAssistant.ToolCalls[0].OfFunction.Function.Name != "ListDir" {
		t.Fatalf("expected tool name ListDir, got %q", params[2].OfAssistant.ToolCalls[0].OfFunction.Function.Name)
	}
}

func TestToOpenAIMessages_InvalidRole(t *testing.T) {
	_, err := toOpenAIMessages([]Message{{Role: Role("invalid"), Content: "x"}})
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestFromOpenAIMessage_RefusalFallbackAndToolCalls(t *testing.T) {
	message := openai.ChatCompletionMessage{
		Refusal: "cannot comply",
		ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
			{
				ID:   "call_1",
				Type: "function",
				Function: openai.ChatCompletionMessageFunctionToolCallFunction{
					Name:      "Read",
					Arguments: `{"file_path":"README.md"}`,
				},
			},
		},
	}

	converted := fromOpenAIMessage(message)
	if converted.Role != RoleAssistant {
		t.Fatalf("expected assistant role, got %q", converted.Role)
	}
	if converted.Content != "cannot comply" {
		t.Fatalf("expected refusal fallback content, got %q", converted.Content)
	}
	if len(converted.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(converted.ToolCalls))
	}
	if converted.ToolCalls[0].Name != "Read" {
		t.Fatalf("expected tool name Read, got %q", converted.ToolCalls[0].Name)
	}
}

func TestToOpenAIToolDefinitions(t *testing.T) {
	defs := toOpenAIToolDefinitions([]ToolDefinition{{
		Name:        "ListDir",
		Description: "List entries",
		Parameters: map[string]any{
			"type": "object",
		},
	}})

	if len(defs) != 1 {
		t.Fatalf("expected one tool definition, got %d", len(defs))
	}
	function := defs[0].OfFunction
	if function == nil {
		t.Fatalf("expected function tool definition")
	}
	if function.Function.Name != "ListDir" {
		t.Fatalf("expected tool name ListDir, got %q", function.Function.Name)
	}
	if function.Function.Description.Value != "List entries" {
		t.Fatalf("expected tool description List entries, got %q", function.Function.Description.Value)
	}
}
