package cli

import (
	"errors"
	"flag"
	"testing"
	"time"
)

func envMap(values map[string]string) func(string) string {
	return func(key string) string {
		if value, ok := values[key]; ok {
			return value
		}
		return ""
	}
}

func TestParseArgs_UsesDefaultsWhenNoFlags(t *testing.T) {
	config, err := ParseArgs([]string{}, envMap(map[string]string{}))
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	if config.LogLevel != "info" {
		t.Fatalf("expected default log level info, got %q", config.LogLevel)
	}
	if config.LogEnabled {
		t.Fatalf("expected default log-enabled false, got true")
	}
	if config.LogSink != "stderr" {
		t.Fatalf("expected default log sink stderr, got %q", config.LogSink)
	}
	if config.LogFile != "" {
		t.Fatalf("expected default log file empty, got %q", config.LogFile)
	}
	if config.Interactive {
		t.Fatalf("expected default interactive false, got true")
	}
	if config.TurnTimeout != 90*time.Second {
		t.Fatalf("expected default turn-timeout 90s, got %s", config.TurnTimeout)
	}
	if config.ToolTimeout != 30*time.Second {
		t.Fatalf("expected default tool-timeout 30s, got %s", config.ToolTimeout)
	}
	if config.MaxTurns != 0 {
		t.Fatalf("expected default max-turns 0, got %d", config.MaxTurns)
	}
	if config.MaxToolCalls != 0 {
		t.Fatalf("expected default max-tool-calls 0, got %d", config.MaxToolCalls)
	}
}

func TestParseArgs_UsesEnvDefaults(t *testing.T) {
	config, err := ParseArgs([]string{}, envMap(map[string]string{
		"LOG_ENABLED":             "yes",
		"LOG_LEVEL":               "debug",
		"SHIMIBOT_LOG_SINK":       "json-file",
		"SHIMIBOT_LOG_FILE":       "/tmp/shimibot.log.jsonl",
		"SHIMIBOT_TURN_TIMEOUT":   "2m",
		"SHIMIBOT_TOOL_TIMEOUT":   "45s",
		"SHIMIBOT_MAX_TURNS":      "7",
		"SHIMIBOT_MAX_TOOL_CALLS": "9",
	}))
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	if !config.LogEnabled {
		t.Fatalf("expected env default log-enabled true, got false")
	}
	if config.LogLevel != "debug" {
		t.Fatalf("expected env default log level debug, got %q", config.LogLevel)
	}
	if config.LogSink != "json-file" {
		t.Fatalf("expected env default log sink json-file, got %q", config.LogSink)
	}
	if config.LogFile != "/tmp/shimibot.log.jsonl" {
		t.Fatalf("expected env default log file /tmp/shimibot.log.jsonl, got %q", config.LogFile)
	}
	if config.TurnTimeout != 2*time.Minute {
		t.Fatalf("expected env default turn-timeout 2m, got %s", config.TurnTimeout)
	}
	if config.ToolTimeout != 45*time.Second {
		t.Fatalf("expected env default tool-timeout 45s, got %s", config.ToolTimeout)
	}
	if config.MaxTurns != 7 {
		t.Fatalf("expected env default max-turns 7, got %d", config.MaxTurns)
	}
	if config.MaxToolCalls != 9 {
		t.Fatalf("expected env default max-tool-calls 9, got %d", config.MaxToolCalls)
	}
}

func TestParseArgs_FlagsOverrideEnvDefaults(t *testing.T) {
	config, err := ParseArgs([]string{"-log-enabled=false", "-log-level=warn", "-log-sink=stdout", "-log-file=/tmp/override.log", "-interactive", "-session=s1", "-p", "hello", "-turn-timeout=75s", "-tool-timeout=12s", "-max-turns=3", "-max-tool-calls=4"}, envMap(map[string]string{
		"LOG_ENABLED":             "true",
		"LOG_LEVEL":               "debug",
		"SHIMIBOT_LOG_SINK":       "json-file",
		"SHIMIBOT_LOG_FILE":       "/tmp/from-env.log",
		"SHIMIBOT_TURN_TIMEOUT":   "2m",
		"SHIMIBOT_TOOL_TIMEOUT":   "45s",
		"SHIMIBOT_MAX_TURNS":      "7",
		"SHIMIBOT_MAX_TOOL_CALLS": "9",
	}))
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	if config.LogEnabled {
		t.Fatalf("expected flag override log-enabled false, got true")
	}
	if config.LogLevel != "warn" {
		t.Fatalf("expected log level warn, got %q", config.LogLevel)
	}
	if config.LogSink != "stdout" {
		t.Fatalf("expected log sink stdout, got %q", config.LogSink)
	}
	if config.LogFile != "/tmp/override.log" {
		t.Fatalf("expected log file /tmp/override.log, got %q", config.LogFile)
	}
	if !config.Interactive {
		t.Fatalf("expected interactive true, got false")
	}
	if config.SessionID != "s1" {
		t.Fatalf("expected session s1, got %q", config.SessionID)
	}
	if config.Prompt != "hello" {
		t.Fatalf("expected prompt hello, got %q", config.Prompt)
	}
	if config.TurnTimeout != 75*time.Second {
		t.Fatalf("expected turn-timeout 75s, got %s", config.TurnTimeout)
	}
	if config.ToolTimeout != 12*time.Second {
		t.Fatalf("expected tool-timeout 12s, got %s", config.ToolTimeout)
	}
	if config.MaxTurns != 3 {
		t.Fatalf("expected max-turns 3, got %d", config.MaxTurns)
	}
	if config.MaxToolCalls != 4 {
		t.Fatalf("expected max-tool-calls 4, got %d", config.MaxToolCalls)
	}
}

func TestParseArgs_ReturnsErrorForInvalidLogLevel(t *testing.T) {
	_, err := ParseArgs([]string{"-log-level=nope"}, envMap(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}

func TestParseArgs_ReturnsErrorForInvalidLogSink(t *testing.T) {
	_, err := ParseArgs([]string{"-log-sink=nope"}, envMap(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for invalid log sink, got nil")
	}
}

func TestParseArgs_ReturnsErrorForMissingLogFileWithJSONSink(t *testing.T) {
	_, err := ParseArgs([]string{"-log-sink=json-file"}, envMap(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for missing log file when using json-file sink")
	}
}

func TestParseArgs_ReturnsErrorForInvalidRuntimeLimits(t *testing.T) {
	if _, err := ParseArgs([]string{"-turn-timeout=0s"}, envMap(map[string]string{})); err == nil {
		t.Fatal("expected error for non-positive turn-timeout")
	}
	if _, err := ParseArgs([]string{"-tool-timeout=-1s"}, envMap(map[string]string{})); err == nil {
		t.Fatal("expected error for non-positive tool-timeout")
	}
	if _, err := ParseArgs([]string{"-max-turns=-1"}, envMap(map[string]string{})); err == nil {
		t.Fatal("expected error for negative max-turns")
	}
	if _, err := ParseArgs([]string{"-max-tool-calls=-2"}, envMap(map[string]string{})); err == nil {
		t.Fatal("expected error for negative max-tool-calls")
	}
}

func TestParseArgs_ReturnsHelpError(t *testing.T) {
	_, err := ParseArgs([]string{"-h"}, envMap(map[string]string{}))
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}
