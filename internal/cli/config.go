package cli

import (
	"fmt"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Prompt      string
	SessionID   string
	Interactive bool
	LogEnabled  bool
	LogLevel    string
	LogSink     string
	LogFile     string
	TurnTimeout  time.Duration
	ToolTimeout  time.Duration
	MaxTurns     int
	MaxToolCalls int
}

func ParseConfig() (Config, error) {
	return ParseArgs(os.Args[1:], os.Getenv)
}

func ParseArgs(args []string, envLookup func(string) string) (Config, error) {
	defaultLogEnabled := parseBoolEnvLookup(envLookup("LOG_ENABLED"), false)
	defaultLogLevel := strings.TrimSpace(envLookup("LOG_LEVEL"))
	if defaultLogLevel == "" {
		defaultLogLevel = "info"
	}
	defaultLogSink := strings.TrimSpace(envLookup("SHIMIBOT_LOG_SINK"))
	if defaultLogSink == "" {
		defaultLogSink = "stderr"
	}
	defaultLogFile := strings.TrimSpace(envLookup("SHIMIBOT_LOG_FILE"))
	defaultTurnTimeout := parseDurationEnvLookup(envLookup("SHIMIBOT_TURN_TIMEOUT"), 90*time.Second)
	defaultToolTimeout := parseDurationEnvLookup(envLookup("SHIMIBOT_TOOL_TIMEOUT"), 30*time.Second)
	defaultMaxTurns := parseIntEnvLookup(envLookup("SHIMIBOT_MAX_TURNS"), 0)
	defaultMaxToolCalls := parseIntEnvLookup(envLookup("SHIMIBOT_MAX_TOOL_CALLS"), 0)

	config := Config{}
	flagSet := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)
	flagSet.StringVar(&config.Prompt, "p", "", "Prompt to send to LLM")
	flagSet.StringVar(&config.SessionID, "session", "", "Session ID used to persist and resume chat history")
	flagSet.BoolVar(&config.Interactive, "interactive", false, "Run in interactive multi-turn mode")
	flagSet.BoolVar(&config.LogEnabled, "log-enabled", defaultLogEnabled, "Enable logging output")
	flagSet.StringVar(&config.LogLevel, "log-level", defaultLogLevel, "Log level: error, warn, info, debug")
	flagSet.StringVar(&config.LogSink, "log-sink", defaultLogSink, "Log sink: stderr, stdout, json-file")
	flagSet.StringVar(&config.LogFile, "log-file", defaultLogFile, "Path for json-file log sink output")
	flagSet.DurationVar(&config.TurnTimeout, "turn-timeout", defaultTurnTimeout, "Maximum duration per prompt turn (e.g. 90s, 2m)")
	flagSet.DurationVar(&config.ToolTimeout, "tool-timeout", defaultToolTimeout, "Maximum duration per tool execution (e.g. 30s, 2m)")
	flagSet.IntVar(&config.MaxTurns, "max-turns", defaultMaxTurns, "Maximum LLM turns per prompt (0 means no limit)")
	flagSet.IntVar(&config.MaxToolCalls, "max-tool-calls", defaultMaxToolCalls, "Maximum tool calls per prompt (0 means no limit)")

	if err := flagSet.Parse(args); err != nil {
		return Config{}, err
	}

	if err := validateConfig(config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func validateConfig(config Config) error {
	level := strings.ToLower(strings.TrimSpace(config.LogLevel))
	switch level {
	case "", "error", "warn", "warning", "info", "debug":
		sink := strings.ToLower(strings.TrimSpace(config.LogSink))
		switch sink {
		case "", "stderr", "stdout", "json-file":
			if sink == "json-file" && strings.TrimSpace(config.LogFile) == "" {
				return fmt.Errorf("invalid value for -log-file: required when -log-sink=json-file")
			}
		default:
			return fmt.Errorf("invalid value for -log-sink: %q (use: stderr, stdout, json-file)", config.LogSink)
		}

		if config.TurnTimeout <= 0 {
			return fmt.Errorf("invalid value for -turn-timeout: must be > 0")
		}
		if config.ToolTimeout <= 0 {
			return fmt.Errorf("invalid value for -tool-timeout: must be > 0")
		}
		if config.MaxTurns < 0 {
			return fmt.Errorf("invalid value for -max-turns: must be >= 0")
		}
		if config.MaxToolCalls < 0 {
			return fmt.Errorf("invalid value for -max-tool-calls: must be >= 0")
		}
		return nil
	default:
		return fmt.Errorf("invalid value for -log-level: %q (use: error, warn, info, debug)", config.LogLevel)
	}
}

func parseBoolEnvLookup(value string, fallback bool) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func parseDurationEnvLookup(value string, fallback time.Duration) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseIntEnvLookup(value string, fallback int) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}
