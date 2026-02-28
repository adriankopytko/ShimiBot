package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type WebSearchOllamaTool struct{}

type webSearchOllamaArgs struct {
	Query      string      `json:"query"`
	MaxResults interface{} `json:"max_results"`
}

func (WebSearchOllamaTool) Name() string {
	return "WebSearchOllama"
}

func (tool WebSearchOllamaTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        tool.Name(),
		Description: "Search the web using your Ollama search endpoint and return a list of search results",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query string",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of search results to return (default 5)",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (WebSearchOllamaTool) Execute(ctx ToolContext, arguments string) (any, error) {
	var args webSearchOllamaArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", fmt.Errorf("query must be a non-empty string")
	}

	maxResults := 5
	switch typed := args.MaxResults.(type) {
	case float64:
		maxResults = int(typed)
	case string:
		if parsed, parseErr := strconv.Atoi(typed); parseErr == nil {
			maxResults = parsed
		}
	}
	if maxResults <= 0 {
		maxResults = 5
	}

	searchURL := strings.TrimSpace(os.Getenv("OLLAMA_WEB_SEARCH_URL"))
	if searchURL == "" {
		return "", fmt.Errorf("set OLLAMA_WEB_SEARCH_URL to your Ollama web search endpoint, and set OLLAMA_WEB_SEARCH_API_KEY with your key")
	}
	if err := EnsureOutboundURLAllowed(BaseContext(ctx), searchURL); err != nil {
		return "", fmt.Errorf("outbound URL blocked: %w", err)
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"query":       query,
		"max_results": maxResults,
	})
	if err != nil {
		return "", fmt.Errorf("error creating request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, searchURL, strings.NewReader(string(requestBody)))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(BaseContext(ctx), EffectiveTimeout(ctx, 20*time.Second))
	defer cancel()
	req = req.WithContext(requestCtx)

	req.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(os.Getenv("OLLAMA_WEB_SEARCH_API_KEY")); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: EffectiveTimeout(ctx, 20*time.Second)}
	resp, err := client.Do(req)
	if err != nil {
		if requestCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("web search request timed out")
		}
		if requestCtx.Err() == context.Canceled {
			return "", fmt.Errorf("web search request cancelled")
		}
		return "", fmt.Errorf("error calling web search endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("error reading web search response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("web search request failed with status %d: %s", resp.StatusCode, string(body))
	}

	results, err := parseSearchResults(body)
	if err != nil {
		return "", fmt.Errorf("error parsing web search results: %w", err)
	}

	return map[string]interface{}{
		"query":   query,
		"results": results,
	}, nil
}

func parseSearchResults(body []byte) ([]WebSearchResult, error) {
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	extractResult := func(item interface{}) WebSearchResult {
		result := WebSearchResult{}
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return result
		}

		if title, ok := itemMap["title"].(string); ok {
			result.Title = title
		} else if name, ok := itemMap["name"].(string); ok {
			result.Title = name
		}

		if url, ok := itemMap["url"].(string); ok {
			result.URL = url
		} else if link, ok := itemMap["link"].(string); ok {
			result.URL = link
		}

		if snippet, ok := itemMap["snippet"].(string); ok {
			result.Snippet = snippet
		} else if description, ok := itemMap["description"].(string); ok {
			result.Snippet = description
		} else if content, ok := itemMap["content"].(string); ok {
			result.Snippet = content
		}

		return result
	}

	collectResults := func(items []interface{}) []WebSearchResult {
		results := make([]WebSearchResult, 0, len(items))
		for _, item := range items {
			result := extractResult(item)
			if strings.TrimSpace(result.URL) != "" {
				results = append(results, result)
			}
		}
		return results
	}

	switch typedPayload := payload.(type) {
	case []interface{}:
		return collectResults(typedPayload), nil
	case map[string]interface{}:
		for _, key := range []string{"results", "items", "data"} {
			if maybeList, ok := typedPayload[key].([]interface{}); ok {
				return collectResults(maybeList), nil
			}
		}
	}

	return []WebSearchResult{}, nil
}
