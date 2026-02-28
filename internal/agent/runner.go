package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/adriankopytko/ShimiBot/internal/llm"
	"github.com/adriankopytko/ShimiBot/internal/tools"
)

var (
	ErrMaxTurnsExceeded       = errors.New("max turns exceeded")
	ErrToolCallBudgetExceeded = errors.New("tool call budget exceeded")
)

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
}

type Policy struct {
	MaxTurns     int
	MaxToolCalls int
}

type Runner struct {
	LLMClient       llm.Client
	Model           string
	ToolDefinitions []llm.ToolDefinition
	ExecuteTool     func(ctx context.Context, correlationID string, toolCall llm.ToolCall) string
	Logger          Logger
	Policy          Policy
}

func (runner Runner) RunPrompt(ctx context.Context, messageHistory *[]llm.Message, prompt string, correlationID string) (string, error) {
	if runner.LLMClient == nil {
		return "", errors.New("agent runner missing llm client")
	}
	if runner.ExecuteTool == nil {
		return "", errors.New("agent runner missing tool executor")
	}

	*messageHistory = append(*messageHistory, llm.Message{
		Role:    llm.RoleUser,
		Content: prompt,
	})

	turnNumber := 1
	toolCallsUsed := 0
	lastAssistantText := ""

	for {
		if err := ctx.Err(); err != nil {
			return lastAssistantText, err
		}

		if runner.Policy.MaxTurns > 0 && turnNumber > runner.Policy.MaxTurns {
			return lastAssistantText, fmt.Errorf("%w: limit=%d", ErrMaxTurnsExceeded, runner.Policy.MaxTurns)
		}

		runner.infoEvent("turn_start", map[string]any{
			"correlation_id": correlationID,
			"turn":           turnNumber,
			"messages":       len(*messageHistory),
		})
		runner.debugf("starting agent turn %d with %d message(s)", turnNumber, len(*messageHistory))
		resp, err := runner.LLMClient.Complete(ctx, llm.CompletionRequest{
			Model:    runner.Model,
			Messages: *messageHistory,
			Tools:    runner.ToolDefinitions,
		})
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", errors.New("no choices in response")
		}

		choice := resp.Choices[0]
		message := choice.Message

		assistantMessage := llm.Message{
			Role:      llm.RoleAssistant,
			Content:   message.Content,
			ToolCalls: make([]llm.ToolCall, 0, len(message.ToolCalls)),
		}
		for _, toolCall := range message.ToolCalls {
			normalizedToolCall := toolCall
			rawArguments := normalizedToolCall.Arguments
			normalizedArguments, valid := tools.NormalizeJSONArguments(rawArguments)
			if valid {
				normalizedToolCall.Arguments = normalizedArguments
				if normalizedArguments != rawArguments {
					runner.warnf("tool call id=%s name=%s had malformed arguments; sanitized before history append", normalizedToolCall.ID, normalizedToolCall.Name)
				}
			} else {
				normalizedToolCall.Arguments = "{}"
				runner.warnf("tool call id=%s name=%s arguments were unrecoverable; replaced with empty object", normalizedToolCall.ID, normalizedToolCall.Name)
			}

			if strings.TrimSpace(assistantMessage.Content) == "" && strings.TrimSpace(message.Content) != "" {
				assistantMessage.Content = message.Content
			}

			if strings.TrimSpace(normalizedToolCall.ID) == "" || strings.TrimSpace(normalizedToolCall.Name) == "" {
				continue
			}
			assistantMessage.ToolCalls = append(assistantMessage.ToolCalls, normalizedToolCall)
		}

		*messageHistory = append(*messageHistory, assistantMessage)

		if strings.TrimSpace(assistantMessage.Content) != "" {
			lastAssistantText = assistantMessage.Content
		} else if strings.TrimSpace(message.Content) != "" {
			lastAssistantText = message.Content
		}

		toolCallCount := len(assistantMessage.ToolCalls)
		runner.infoEvent("turn_end", map[string]any{
			"correlation_id": correlationID,
			"turn":           turnNumber,
			"finish_reason":  choice.FinishReason,
			"tool_calls":     toolCallCount,
		})
		runner.infof("turn %d finished with reason=%s tool_calls=%d", turnNumber, choice.FinishReason, toolCallCount)
		if choice.FinishReason == "stop" || toolCallCount == 0 {
			runner.debugf("agent loop stopping on turn %d", turnNumber)
			break
		}

		if runner.Policy.MaxToolCalls > 0 && toolCallsUsed+toolCallCount > runner.Policy.MaxToolCalls {
			return lastAssistantText, fmt.Errorf("%w: limit=%d used=%d requested=%d", ErrToolCallBudgetExceeded, runner.Policy.MaxToolCalls, toolCallsUsed, toolCallCount)
		}

		for _, toolCall := range assistantMessage.ToolCalls {
			if err := ctx.Err(); err != nil {
				return lastAssistantText, err
			}

			runner.infoEvent("tool_start", map[string]any{
				"correlation_id": correlationID,
				"turn":           turnNumber,
				"tool_call_id":   toolCall.ID,
				"tool":           toolCall.Name,
			})
			runner.infof("executing tool call id=%s name=%s", toolCall.ID, toolCall.Name)
			toolResponse := runner.ExecuteTool(ctx, correlationID, toolCall)
			runner.infoEvent("tool_end", map[string]any{
				"correlation_id":  correlationID,
				"turn":            turnNumber,
				"tool_call_id":    toolCall.ID,
				"tool":            toolCall.Name,
				"response_bytes":  len(toolResponse),
			})
			runner.debugf("tool call id=%s completed with %d byte(s) response", toolCall.ID, len(toolResponse))
			*messageHistory = append(*messageHistory, llm.Message{
				Role:       llm.RoleTool,
				Content:    toolResponse,
				ToolCallID: toolCall.ID,
			})
		}

		toolCallsUsed += toolCallCount
		turnNumber++
	}

	runner.debugf("assistant completed and produced final response")
	return lastAssistantText, nil
}

func (runner Runner) debugf(format string, args ...interface{}) {
	if runner.Logger == nil {
		return
	}
	runner.Logger.Debugf(format, args...)
}

func (runner Runner) infof(format string, args ...interface{}) {
	if runner.Logger == nil {
		return
	}
	runner.Logger.Infof(format, args...)
}

func (runner Runner) warnf(format string, args ...interface{}) {
	if runner.Logger == nil {
		return
	}
	runner.Logger.Warnf(format, args...)
}

func (runner Runner) infoEvent(event string, fields map[string]any) {
	if runner.Logger == nil {
		return
	}
	runner.Logger.Infof("event=%s %s", event, formatFields(fields))
}

func formatFields(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+formatFieldValue(fields[key]))
	}
	return strings.Join(parts, " ")
}

func formatFieldValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.ReplaceAll(typed, " ", "_")
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", typed)
	}
}
