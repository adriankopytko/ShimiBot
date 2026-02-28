package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type OpenAIClient struct {
	client openai.Client
}

func NewOpenAIClient(apiKey, baseURL string) *OpenAIClient {
	return &OpenAIClient{client: openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL))}
}

func (client *OpenAIClient) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	messageParams, err := toOpenAIMessages(request.Messages)
	if err != nil {
		return CompletionResponse{}, err
	}

	toolDefinitions := toOpenAIToolDefinitions(request.Tools)
	response, err := client.client.Chat.Completions.New(ctx,
		openai.ChatCompletionNewParams{
			Model:    request.Model,
			Messages: messageParams,
			Tools:    toolDefinitions,
		},
	)
	if err != nil {
		return CompletionResponse{}, err
	}

	return fromOpenAIResponse(response), nil
}

func toOpenAIToolDefinitions(definitions []ToolDefinition) []openai.ChatCompletionToolUnionParam {
	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(definitions))
	for _, definition := range definitions {
		parameters := openai.FunctionParameters{}
		for key, value := range definition.Parameters {
			parameters[key] = value
		}

		tools = append(tools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        definition.Name,
			Description: openai.String(definition.Description),
			Parameters:  parameters,
		}))
	}
	return tools
}

func toOpenAIMessages(messages []Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case RoleSystem:
			result = append(result, openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{OfString: openai.String(message.Content)},
				},
			})
		case RoleUser:
			result = append(result, openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{OfString: openai.String(message.Content)},
				},
			})
		case RoleAssistant:
			assistant := openai.ChatCompletionAssistantMessageParam{}
			if strings.TrimSpace(message.Content) != "" {
				assistant.Content = openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(message.Content)}
			}
			if len(message.ToolCalls) > 0 {
				assistant.ToolCalls = make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(message.ToolCalls))
				for _, toolCall := range message.ToolCalls {
					assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: toolCall.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      toolCall.Name,
								Arguments: toolCall.Arguments,
							},
						},
					})
				}
			}
			result = append(result, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
		case RoleTool:
			result = append(result, openai.ChatCompletionMessageParamUnion{
				OfTool: &openai.ChatCompletionToolMessageParam{
					Role:       "tool",
					ToolCallID: message.ToolCallID,
					Content: openai.ChatCompletionToolMessageParamContentUnion{
						OfString: openai.String(message.Content),
					},
				},
			})
		default:
			return nil, fmt.Errorf("unsupported message role %q", message.Role)
		}
	}
	return result, nil
}

func fromOpenAIResponse(response *openai.ChatCompletion) CompletionResponse {
	if response == nil {
		return CompletionResponse{}
	}

	choices := make([]Choice, 0, len(response.Choices))
	for _, choice := range response.Choices {
		choices = append(choices, Choice{
			FinishReason: choice.FinishReason,
			Message:      fromOpenAIMessage(choice.Message),
		})
	}
	return CompletionResponse{Choices: choices}
}

func fromOpenAIMessage(message openai.ChatCompletionMessage) Message {
	content := strings.TrimSpace(message.Content)
	if content == "" {
		content = strings.TrimSpace(message.Refusal)
	}

	toolCalls := make([]ToolCall, 0, len(message.ToolCalls))
	for _, toolCall := range message.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: toolCall.Function.Arguments,
		})
	}

	return Message{
		Role:      RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
	}
}
