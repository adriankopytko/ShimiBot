package appcore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
	"github.com/adriankopytko/ShimiBot/internal/tools"
	"github.com/joho/godotenv"
)

type LLMConfig struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

func BuildSystemPrompt(now time.Time) string {
	currentDate := now.Format("January 2, 2006")
	return fmt.Sprintf("You are a helpful personal assistant. Today's date is %s. Be concise, accurate, and practical. Use available tools when needed and clearly explain your reasoning and limits.", currentDate)
}

func LoadEnvFilesIfPresent(paths []string, logger Logger) {
	for _, path := range paths {
		_, statErr := os.Stat(path)
		if statErr != nil {
			if !errors.Is(statErr, os.ErrNotExist) {
				logger.Warnf("failed to check env file %s: %v", path, statErr)
				fmt.Fprintf(os.Stderr, "warning: failed to check env file %s: %v\n", path, statErr)
			}
			continue
		}

		if err := godotenv.Load(path); err != nil {
			logger.Warnf("failed to load env file %s: %v", path, err)
			fmt.Fprintf(os.Stderr, "warning: failed to load env file %s: %v\n", path, err)
		} else {
			logger.Debugf("loaded env file: %s", path)
		}
	}
}

func ResolveLLMConfig(logger Logger) (LLMConfig, error) {
	provider := "openrouter"
	logger.Debugf("resolved provider=%s", provider)

	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	baseURL := strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("AI_MODEL"))

	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if model == "" {
		model = "anthropic/claude-haiku-4.5"
	}

	if apiKey == "" {
		logger.Errorf("openrouter api key not found")
		return LLMConfig{}, errors.New("missing API key: set OPENROUTER_API_KEY")
	}

	logger.Debugf("llm config resolved (provider=%s model=%s base_url=%s)", provider, model, baseURL)

	return LLMConfig{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  baseURL,
		Model:    model,
	}, nil
}

func DispatchToolCall(logger Logger, toolRegistry *tools.Registry, toolContext tools.ToolContext, toolCall llm.ToolCall) string {
	toolName := toolCall.Name
	logger.Debugf("event=tool_dispatch correlation_id=%s tool=%s", toolContext.CorrelationID, toolName)

	if toolRegistry != nil {
		output, matched := toolRegistry.Execute(toolCall, toolContext)
		if matched {
			logger.Debugf("event=tool_dispatch_complete correlation_id=%s tool=%s", toolContext.CorrelationID, toolName)
			return output
		}
	}

	logger.Warnf("event=tool_unknown correlation_id=%s tool=%s", toolContext.CorrelationID, toolName)
	return tools.ErrorEnvelope(fmt.Sprintf("unknown tool '%s'", toolName), map[string]any{"tool": toolName})
}

func NewCorrelationID() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("corr-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("corr-%d-%s", time.Now().UnixNano(), hex.EncodeToString(bytes))
}
