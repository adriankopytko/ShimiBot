package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

func TestRunPrompt_SingleTurnStop(t *testing.T) {
	history := []llm.Message{{Role: llm.RoleSystem, Content: "system"}}

	llmClient := &queuedClient{responses: []llm.CompletionResponse{responseWithText("stop", "hello")}}
	toolExecutions := 0
	runner := Runner{
		LLMClient: llmClient,
		Model:     "test-model",
		ExecuteTool: func(ctx context.Context, correlationID string, toolCall llm.ToolCall) string {
			toolExecutions++
			return "{}"
		},
	}

	responseText, err := runner.RunPrompt(context.Background(), &history, "hi", "corr-1")
	if err != nil {
		t.Fatalf("RunPrompt returned error: %v", err)
	}
	if responseText != "hello" {
		t.Fatalf("expected response text hello, got %q", responseText)
	}
	if toolExecutions != 0 {
		t.Fatalf("expected no tool executions, got %d", toolExecutions)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	if history[1].Role != llm.RoleUser || history[1].Content != "hi" {
		t.Fatalf("expected user prompt appended")
	}
	if history[2].Role != llm.RoleAssistant || history[2].Content != "hello" {
		t.Fatalf("expected assistant response appended")
	}
}

func TestRunPrompt_ToolCallThenStop(t *testing.T) {
	history := []llm.Message{{Role: llm.RoleSystem, Content: "system"}}

	llmClient := &queuedClient{responses: []llm.CompletionResponse{
		responseWithToolCalls(sampleToolCall("call_1", "ListDir", "{}")),
		responseWithText("stop", "done"),
	}}
	toolExecutions := 0
	runner := Runner{
		LLMClient: llmClient,
		Model:     "test-model",
		ExecuteTool: func(ctx context.Context, correlationID string, toolCall llm.ToolCall) string {
			toolExecutions++
			if toolCall.Name != "ListDir" {
				t.Fatalf("unexpected tool name %q", toolCall.Name)
			}
			return `{"entries":[]}`
		},
	}

	responseText, err := runner.RunPrompt(context.Background(), &history, "use tool", "corr-2")
	if err != nil {
		t.Fatalf("RunPrompt returned error: %v", err)
	}
	if responseText != "done" {
		t.Fatalf("expected response text done, got %q", responseText)
	}
	if toolExecutions != 1 {
		t.Fatalf("expected one tool execution, got %d", toolExecutions)
	}
	if len(history) != 5 {
		t.Fatalf("expected 5 history entries, got %d", len(history))
	}
	if history[3].Role != llm.RoleTool || history[3].ToolCallID != "call_1" {
		t.Fatalf("expected tool result appended")
	}
}

func TestRunPrompt_EnforcesToolBudget(t *testing.T) {
	history := []llm.Message{}
	llmClient := &queuedClient{responses: []llm.CompletionResponse{
		responseWithToolCalls(
			sampleToolCall("call_1", "ListDir", "{}"),
			sampleToolCall("call_2", "Read", `{"path":"README.md"}`),
		),
	}}
	toolExecutions := 0
	runner := Runner{
		LLMClient: llmClient,
		Model:     "test-model",
		ExecuteTool: func(ctx context.Context, correlationID string, toolCall llm.ToolCall) string {
			toolExecutions++
			return "{}"
		},
		Policy: Policy{MaxToolCalls: 1},
	}

	_, err := runner.RunPrompt(context.Background(), &history, "budget", "corr-3")
	if !errors.Is(err, ErrToolCallBudgetExceeded) {
		t.Fatalf("expected ErrToolCallBudgetExceeded, got %v", err)
	}
	if toolExecutions != 0 {
		t.Fatalf("expected no tool execution after budget failure, got %d", toolExecutions)
	}
}

func TestRunPrompt_EnforcesMaxTurns(t *testing.T) {
	history := []llm.Message{}
	llmClient := &queuedClient{responses: []llm.CompletionResponse{
		responseWithToolCalls(sampleToolCall("call_1", "ListDir", "{}")),
	}}
	toolExecutions := 0
	runner := Runner{
		LLMClient: llmClient,
		Model:     "test-model",
		ExecuteTool: func(ctx context.Context, correlationID string, toolCall llm.ToolCall) string {
			toolExecutions++
			return "{}"
		},
		Policy: Policy{MaxTurns: 1},
	}

	_, err := runner.RunPrompt(context.Background(), &history, "turn-limit", "corr-4")
	if !errors.Is(err, ErrMaxTurnsExceeded) {
		t.Fatalf("expected ErrMaxTurnsExceeded, got %v", err)
	}
	if toolExecutions != 1 {
		t.Fatalf("expected first-turn tool execution before max-turn exit, got %d", toolExecutions)
	}
}

func TestRunPrompt_ContextCancelled(t *testing.T) {
	history := []llm.Message{}
	llmClient := &queuedClient{responses: []llm.CompletionResponse{responseWithText("stop", "unused")}}
	runner := Runner{
		LLMClient: llmClient,
		Model:     "test-model",
		ExecuteTool: func(ctx context.Context, correlationID string, toolCall llm.ToolCall) string {
			return "{}"
		},
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.RunPrompt(cancelledCtx, &history, "cancel", "corr-cancel")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

type queuedClient struct {
	responses []llm.CompletionResponse
	index     int
}

func (client *queuedClient) Complete(ctx context.Context, request llm.CompletionRequest) (llm.CompletionResponse, error) {
	if client.index >= len(client.responses) {
		return llm.CompletionResponse{}, errors.New("no queued response")
	}
	response := client.responses[client.index]
	client.index++
	return response, nil
}

func responseWithText(finishReason, content string) llm.CompletionResponse {
	return llm.CompletionResponse{
		Choices: []llm.Choice{{
			FinishReason: finishReason,
			Message:      llm.Message{Role: llm.RoleAssistant, Content: content},
		}},
	}
}

func responseWithToolCalls(toolCalls ...llm.ToolCall) llm.CompletionResponse {
	return llm.CompletionResponse{
		Choices: []llm.Choice{{
			FinishReason: "tool_calls",
			Message:      llm.Message{Role: llm.RoleAssistant, ToolCalls: toolCalls},
		}},
	}
}

func sampleToolCall(id, name, arguments string) llm.ToolCall {
	return llm.ToolCall{ID: id, Name: name, Arguments: arguments}
}
