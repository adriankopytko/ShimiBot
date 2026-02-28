package appcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type captureSink struct {
	entries []LogEntry
}

func (sink *captureSink) Write(entry LogEntry) error {
	sink.entries = append(sink.entries, entry)
	return nil
}

func TestLogger_ParsesStructuredEventMessage(t *testing.T) {
	sink := &captureSink{}
	logger := Logger{enabled: true, level: LogLevelInfo, sink: sink}

	logger.Infof("event=turn_complete correlation_id=corr-123 response_chars=42 ok=true")

	if len(sink.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sink.entries))
	}
	entry := sink.entries[0]
	if entry.Event != "turn_complete" {
		t.Fatalf("expected event turn_complete, got %q", entry.Event)
	}
	if entry.SchemaVersion != EventSchemaVersion {
		t.Fatalf("expected schema version %q, got %q", EventSchemaVersion, entry.SchemaVersion)
	}
	if value, ok := entry.Fields["correlation_id"].(string); !ok || value != "corr-123" {
		t.Fatalf("expected correlation_id corr-123, got %#v", entry.Fields["correlation_id"])
	}
	if value, ok := entry.Fields["response_chars"].(int); !ok || value != 42 {
		t.Fatalf("expected response_chars 42, got %#v", entry.Fields["response_chars"])
	}
	if value, ok := entry.Fields["ok"].(bool); !ok || !value {
		t.Fatalf("expected ok true, got %#v", entry.Fields["ok"])
	}
}

func TestLogger_RespectsLogLevel(t *testing.T) {
	sink := &captureSink{}
	logger := Logger{enabled: true, level: LogLevelWarn, sink: sink}

	logger.Infof("info message")
	logger.Warnf("warn message")

	if len(sink.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sink.entries))
	}
	if sink.entries[0].Level != LogLevelWarn {
		t.Fatalf("expected warn level entry, got %s", sink.entries[0].Level.String())
	}
}

func TestNewLogger_RequiresFilePathForJSONFileSink(t *testing.T) {
	_, err := NewLogger(true, "info", LoggerSinkConfig{Sink: "json-file"})
	if err == nil {
		t.Fatal("expected error when json-file sink has no path")
	}
}

func TestJSONFileSink_WritesSchemaVersionedEventRecord(t *testing.T) {
	logFilePath := filepath.Join(t.TempDir(), "shimibot.jsonl")

	logger, err := NewLogger(true, "info", LoggerSinkConfig{Sink: "json-file", FilePath: logFilePath})
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}

	logger.Infof("event=tool_end correlation_id=c-1 tool=Read response_bytes=17")

	payload, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("failed reading log file: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected log payload, got empty file")
	}

	var record struct {
		SchemaVersion string         `json:"schema_version"`
		Level         string         `json:"level"`
		Event         string         `json:"event"`
		Fields        map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(payload, &record); err != nil {
		t.Fatalf("failed to decode log record: %v", err)
	}
	if record.SchemaVersion != EventSchemaVersion {
		t.Fatalf("expected schema_version %q, got %q", EventSchemaVersion, record.SchemaVersion)
	}
	if record.Level != "info" {
		t.Fatalf("expected level info, got %q", record.Level)
	}
	if record.Event != "tool_end" {
		t.Fatalf("expected event tool_end, got %q", record.Event)
	}
	if record.Fields["correlation_id"] != "c-1" {
		t.Fatalf("expected correlation_id c-1, got %#v", record.Fields["correlation_id"])
	}
}
