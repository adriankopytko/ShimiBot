package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/appcore"
	"github.com/adriankopytko/ShimiBot/internal/agent"
	"github.com/adriankopytko/ShimiBot/internal/cli"
	"github.com/adriankopytko/ShimiBot/internal/llm"
	"github.com/adriankopytko/ShimiBot/internal/session"
	"github.com/adriankopytko/ShimiBot/internal/tools"
)

func main() {
	appcore.LoadEnvFilesIfPresent([]string{".env", "app/.env"}, appcore.Logger{})

	cliConfig, err := cli.ParseConfig()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	appLogger, err := appcore.NewLogger(cliConfig.LogEnabled, cliConfig.LogLevel, appcore.LoggerSinkConfig{
		Sink:     cliConfig.LogSink,
		FilePath: cliConfig.LogFile,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	appLogger.Infof("startup complete (log_enabled=%t, log_level=%s, log_sink=%s)", cliConfig.LogEnabled, strings.ToLower(cliConfig.LogLevel), strings.ToLower(cliConfig.LogSink))

	if strings.TrimSpace(cliConfig.Prompt) == "" && !cliConfig.Interactive {
		cliConfig.Interactive = true
	}

	if cliConfig.Interactive && strings.TrimSpace(cliConfig.SessionID) == "" {
		cliConfig.SessionID = session.DefaultSessionID(time.Now())
		appLogger.Infof("created session id=%s", cliConfig.SessionID)
	}

	sessionStore := session.NewJSONFileStore()

	llmConfig, err := appcore.ResolveLLMConfig(appLogger)
	if err != nil {
		appLogger.Errorf("failed resolving llm config: %v", err)
		panic(err.Error())
	}
	toolRegistry := tools.DefaultRegistry()
	appLogger.Infof("using provider=%s model=%s base_url=%s", llmConfig.Provider, llmConfig.Model, llmConfig.BaseURL)

	workingDir, wdErr := os.Getwd()
	if wdErr != nil {
		appLogger.Warnf("failed to get working directory: %v", wdErr)
		workingDir = "."
	}
	toolContext := tools.ToolContext{
		CWD:         workingDir,
		AllowedRoot: workingDir,
		Timeout:     cliConfig.ToolTimeout,
		Logger:      appLogger,
	}

	llmClient := llm.NewOpenAIClient(llmConfig.APIKey, llmConfig.BaseURL)
	agentRunner := agent.Runner{
		LLMClient:       llmClient,
		Model:           llmConfig.Model,
		ToolDefinitions: toolRegistry.Definitions(),
		ExecuteTool: func(ctx context.Context, correlationID string, toolCall llm.ToolCall) string {
			turnToolContext := toolContext
			turnToolContext.Context = ctx
			turnToolContext.CorrelationID = correlationID
			return appcore.DispatchToolCall(appLogger, toolRegistry, turnToolContext, toolCall)
		},
		Logger: appLogger,
		Policy: agent.Policy{
			MaxTurns:     cliConfig.MaxTurns,
			MaxToolCalls: cliConfig.MaxToolCalls,
		},
	}

	messageHistory, err := sessionStore.Load(cliConfig.SessionID)
	if err != nil {
		appLogger.Errorf("failed loading session history: %v", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(messageHistory) == 0 {
		systemPrompt := appcore.BuildSystemPrompt(time.Now())
		messageHistory = append(messageHistory, llm.Message{
			Role:    llm.RoleSystem,
			Content: systemPrompt,
		})
		appLogger.Debugf("system prompt initialized with current date")
	}

	runAgentTurn := func(prompt string) (string, error) {
		correlationID := appcore.NewCorrelationID()
		appLogger.Infof("event=turn_request correlation_id=%s prompt_chars=%d", correlationID, len(prompt))

		turnCtx, cancel := context.WithTimeout(context.Background(), cliConfig.TurnTimeout)
		defer cancel()

		responseText, runErr := agentRunner.RunPrompt(turnCtx, &messageHistory, prompt, correlationID)
		if runErr != nil {
			appLogger.Errorf("event=turn_error correlation_id=%s err=%v", correlationID, runErr)
			return "", runErr
		}

		appLogger.Infof("event=turn_complete correlation_id=%s response_chars=%d", correlationID, len(responseText))
		return responseText, nil
	}

	if strings.TrimSpace(cliConfig.Prompt) != "" {
		appLogger.Debugf("received prompt with %d characters", len(cliConfig.Prompt))
		responseText, runErr := runAgentTurn(cliConfig.Prompt)
		if runErr != nil {
			appLogger.Errorf("prompt run failed: %v", runErr)
			fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
			os.Exit(1)
		}
		if strings.TrimSpace(cliConfig.SessionID) != "" {
			if saveErr := sessionStore.Save(cliConfig.SessionID, messageHistory); saveErr != nil {
				appLogger.Errorf("failed saving session history: %v", saveErr)
				fmt.Fprintf(os.Stderr, "warning: failed saving session history: %v\n", saveErr)
			}
		}
		fmt.Print(responseText)
		if !cliConfig.Interactive {
			os.Exit(0)
		}
		fmt.Println()
	}

	if cliConfig.Interactive {
		runErr := cli.RunInteractive(cliConfig.SessionID, func(input string) (string, error) {
			responseText, promptErr := runAgentTurn(input)
			if promptErr != nil {
				appLogger.Errorf("interactive prompt failed: %v", promptErr)
				return "", promptErr
			}

			if strings.TrimSpace(cliConfig.SessionID) != "" {
				if saveErr := sessionStore.Save(cliConfig.SessionID, messageHistory); saveErr != nil {
					appLogger.Errorf("failed saving session history: %v", saveErr)
					fmt.Fprintf(os.Stderr, "warning: failed saving session history: %v\n", saveErr)
				}
			}

			return responseText, nil
		})
		if runErr != nil {
			appLogger.Errorf("interactive input failed: %v", runErr)
			fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
