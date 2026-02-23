package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))
	messageHistory := []openai.ChatCompletionMessageParamUnion{}
	messageHistory = append(messageHistory, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(prompt),
			},
		},
	})

	for {
		resp, err := generateChatCompletion(client, messageHistory)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(resp.Choices) == 0 {
			panic("No choices in response")
		}

		message := resp.Choices[0].Message

		messageHistory = append(messageHistory, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				Content: openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: message.ToAssistantMessageParam().Content.OfString,
				},
				ToolCalls: message.ToAssistantMessageParam().ToolCalls,
			},
		})

		if resp.Choices[0].FinishReason == "stop" && len(message.ToolCalls) == 0 {
			break
		}

		if len(message.ToolCalls) > 0 {
			for _, toolCall := range message.ToolCalls {
				toolResponse := handleToolCall(toolCall)
				messageHistory = append(messageHistory, openai.ChatCompletionMessageParamUnion{
					OfTool: &openai.ChatCompletionToolMessageParam{
						Role: "tool",
						Content: openai.ChatCompletionToolMessageParamContentUnion{
							OfString: openai.String(toolResponse),
						},
						ToolCallID: toolCall.ID,
					},
				})
			}
		}
	}

	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	fmt.Print(messageHistory[len(messageHistory)-1].OfAssistant.Content.OfString)

	os.Exit(0)
}

func handleToolCall(toolCall openai.ChatCompletionMessageToolCallUnion) string {
	tool_name := toolCall.Function.Name

	if tool_name == "Read" {
		content := readFileContent(toolCall)
		return string(content)
	}

	return ""
}

func readFileContent(toolCall openai.ChatCompletionMessageToolCallUnion) []byte {
	var args map[string]interface{}

	err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	content, err := os.ReadFile(args["file_path"].(string))

	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
		os.Exit(1)
	}
	return content
}

func generateChatCompletion(client openai.Client, messages []openai.ChatCompletionMessageParamUnion) (*openai.ChatCompletion, error) {
	resp, err := client.Chat.Completions.New(context.Background(),
		openai.ChatCompletionNewParams{
			Model:    "anthropic/claude-haiku-4.5",
			Messages: messages,
			Tools: []openai.ChatCompletionToolUnionParam{
				openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
					Name:        "Read",
					Description: openai.String("Read and return the contents of a file"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]any{
							"file_path": map[string]any{
								"type":        "string",
								"description": "The path to the file to read",
							},
						},
						"required": []string{"file_path"},
					},
				}),
			},
		},
	)
	return resp, err
}
