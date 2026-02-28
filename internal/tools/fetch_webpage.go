package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type FetchWebPageTool struct{}

type fetchWebPageArgs struct {
	URL string `json:"url"`
}

var (
	scriptTagRegex = regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	styleTagRegex  = regexp.MustCompile(`(?is)<style.*?>.*?</style>`)
	htmlTagRegex   = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRegex     = regexp.MustCompile(`\s+`)
)

func (FetchWebPageTool) Name() string {
	return "FetchWebPage"
}

func (tool FetchWebPageTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        tool.Name(),
		Description: "Fetch a webpage by URL and return extracted text content",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Fully-qualified URL to fetch",
				},
			},
			"required": []string{"url"},
		},
	}
}

func (FetchWebPageTool) Execute(ctx ToolContext, arguments string) (any, error) {
	var args fetchWebPageArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}

	pageURL := strings.TrimSpace(args.URL)
	if pageURL == "" {
		return "", fmt.Errorf("url must be a non-empty string")
	}
	if err := EnsureOutboundURLAllowed(BaseContext(ctx), pageURL); err != nil {
		return "", fmt.Errorf("outbound URL blocked: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("User-Agent", "ShimiBot/1.0")

	requestCtx, cancel := context.WithTimeout(BaseContext(ctx), EffectiveTimeout(ctx, 20*time.Second))
	defer cancel()
	req = req.WithContext(requestCtx)

	client := &http.Client{Timeout: EffectiveTimeout(ctx, 20*time.Second)}
	resp, err := client.Do(req)
	if err != nil {
		if requestCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("webpage request timed out")
		}
		if requestCtx.Err() == context.Canceled {
			return "", fmt.Errorf("webpage request cancelled")
		}
		return "", fmt.Errorf("error fetching webpage: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("error reading webpage response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("webpage request failed with status %d", resp.StatusCode)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	textContent := string(body)
	if strings.Contains(contentType, "text/html") {
		textContent = extractTextFromHTML(textContent)
	}

	return map[string]interface{}{
		"url":          pageURL,
		"content_type": contentType,
		"content":      strings.TrimSpace(textContent),
	}, nil
}

func extractTextFromHTML(content string) string {
	withoutScripts := scriptTagRegex.ReplaceAllString(content, " ")
	withoutStyles := styleTagRegex.ReplaceAllString(withoutScripts, " ")
	withoutTags := htmlTagRegex.ReplaceAllString(withoutStyles, " ")
	decoded := html.UnescapeString(withoutTags)
	return spaceRegex.ReplaceAllString(decoded, " ")
}
