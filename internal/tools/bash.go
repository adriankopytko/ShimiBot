package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type BashTool struct{}

type bashArgs struct {
	Command string `json:"command"`
}

var blockedCommandPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-rf\s+/`),
	regexp.MustCompile(`(?i)\bshutdown\b`),
	regexp.MustCompile(`(?i)\breboot\b`),
	regexp.MustCompile(`(?i)\bpoweroff\b`),
	regexp.MustCompile(`(?i)\bmkfs(\.|\s)`),
	regexp.MustCompile(`:\s*\(\s*\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`),
}

const (
	bashAllowlistEnv = "SHIMIBOT_BASH_ALLOWLIST"
	bashDenylistEnv  = "SHIMIBOT_BASH_DENYLIST"
)

func (BashTool) Name() string {
	return "Bash"
}

func (tool BashTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        tool.Name(),
		Description: "Execute a shell command",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			"required": []string{"command"},
		},
	}
}

func (BashTool) Execute(ctx ToolContext, arguments string) (any, error) {
	var args bashArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("error parsing arguments: %w", err)
	}

	command := strings.TrimSpace(args.Command)
	if command == "" {
		return "", fmt.Errorf("command must be a non-empty string")
	}
	if err := validateCommandPolicy(command); err != nil {
		return "", err
	}

	if ctx.Logger != nil {
		ctx.Logger.Debugf("BashTool executing command in cwd=%s", strings.TrimSpace(ctx.CWD))
	}

	timeout := EffectiveTimeout(ctx, 30*time.Second)
	commandCtx, cancel := context.WithTimeout(BaseContext(ctx), timeout)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, "bash", "-c", command)
	if strings.TrimSpace(ctx.CWD) != "" {
		cmd.Dir = ctx.CWD
	}

	output, err := cmd.CombinedOutput()
	if commandCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("bash command timed out after %s", timeout)
	}
	if err != nil {
		return "", fmt.Errorf("error executing bash command: %w", err)
	}

	return map[string]any{
		"command": command,
		"output":  string(output),
	}, nil
}

func validateCommandPolicy(command string) error {
	for _, pattern := range blockedCommandPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command blocked by policy")
		}
	}

	denylistPatterns, err := compilePolicyPatterns(os.Getenv(bashDenylistEnv))
	if err != nil {
		return fmt.Errorf("invalid %s: %w", bashDenylistEnv, err)
	}
	for _, pattern := range denylistPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command blocked by denylist policy")
		}
	}

	allowlistPatterns, err := compilePolicyPatterns(os.Getenv(bashAllowlistEnv))
	if err != nil {
		return fmt.Errorf("invalid %s: %w", bashAllowlistEnv, err)
	}
	if len(allowlistPatterns) > 0 {
		for _, pattern := range allowlistPatterns {
			if pattern.MatchString(command) {
				return nil
			}
		}
		return fmt.Errorf("command blocked by allowlist policy")
	}

	return nil
}

func compilePolicyPatterns(raw string) ([]*regexp.Regexp, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []*regexp.Regexp{}, nil
	}

	parts := splitPolicyList(raw)
	patterns := make([]*regexp.Regexp, 0, len(parts))
	for _, part := range parts {
		compiled, err := regexp.Compile(part)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", part, err)
		}
		patterns = append(patterns, compiled)
	}
	return patterns, nil
}

func splitPolicyList(raw string) []string {
	segments := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n':
			return true
		default:
			return false
		}
	})

	values := make([]string, 0, len(segments))
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}
