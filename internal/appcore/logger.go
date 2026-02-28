package appcore

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

type Logger struct {
	enabled bool
	level   LogLevel
	sink    LogSink
}

const EventSchemaVersion = "v1"

type LogSink interface {
	Write(entry LogEntry) error
}

type LogEntry struct {
	Timestamp     time.Time
	Level         LogLevel
	Message       string
	SchemaVersion string
	Event         string
	Fields        map[string]any
}

type LoggerSinkConfig struct {
	Sink     string
	FilePath string
}

type textSink struct {
	writer io.Writer
}

type jsonFileSink struct {
	mu   sync.Mutex
	file *os.File
}

type jsonLogRecord struct {
	SchemaVersion string         `json:"schema_version"`
	Timestamp     string         `json:"timestamp"`
	Level         string         `json:"level"`
	Message       string         `json:"message,omitempty"`
	Event         string         `json:"event,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
}

func (level LogLevel) String() string {
	switch level {
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

func parseLogLevel(level string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return LogLevelError, nil
	case "warn", "warning":
		return LogLevelWarn, nil
	case "info", "":
		return LogLevelInfo, nil
	case "debug":
		return LogLevelDebug, nil
	default:
		return LogLevelInfo, fmt.Errorf("invalid log level %q (use: error, warn, info, debug)", level)
	}
}

func NewLogSink(config LoggerSinkConfig) (LogSink, error) {
	sink := strings.ToLower(strings.TrimSpace(config.Sink))
	if sink == "" {
		sink = "stderr"
	}

	switch sink {
	case "stderr":
		return textSink{writer: os.Stderr}, nil
	case "stdout":
		return textSink{writer: os.Stdout}, nil
	case "json-file":
		filePath := strings.TrimSpace(config.FilePath)
		if filePath == "" {
			return nil, fmt.Errorf("log sink json-file requires a file path")
		}
		dir := filepath.Dir(filePath)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("failed creating log directory %q: %w", dir, err)
			}
		}
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("failed opening log file %q: %w", filePath, err)
		}
		return &jsonFileSink{file: file}, nil
	default:
		return nil, fmt.Errorf("invalid log sink %q (use: stderr, stdout, json-file)", config.Sink)
	}
}

func NewLogger(enabled bool, level string, sinkConfig LoggerSinkConfig) (Logger, error) {
	parsedLevel, err := parseLogLevel(level)
	if err != nil {
		return Logger{}, err
	}

	sink, err := NewLogSink(sinkConfig)
	if err != nil {
		return Logger{}, err
	}

	return Logger{enabled: enabled, level: parsedLevel, sink: sink}, nil
}

func (logger Logger) logf(level LogLevel, format string, args ...interface{}) {
	if !logger.enabled || level > logger.level {
		return
	}
	message := fmt.Sprintf(format, args...)

	entry := LogEntry{
		Timestamp:     time.Now(),
		Level:         level,
		Message:       message,
		SchemaVersion: EventSchemaVersion,
	}
	if eventName, fields, ok := parseEventMessage(message); ok {
		entry.Event = eventName
		entry.Fields = fields
	}

	sink := logger.sink
	if sink == nil {
		sink = textSink{writer: os.Stderr}
	}

	if err := sink.Write(entry); err != nil {
		fmt.Fprintf(os.Stderr, "%s [WARN] failed to write log entry: %v\n", time.Now().Format(time.RFC3339), err)
	}
}

func (logger Logger) Errorf(format string, args ...interface{}) {
	logger.logf(LogLevelError, format, args...)
}

func (logger Logger) Warnf(format string, args ...interface{}) {
	logger.logf(LogLevelWarn, format, args...)
}

func (logger Logger) Infof(format string, args ...interface{}) {
	logger.logf(LogLevelInfo, format, args...)
}

func (logger Logger) Debugf(format string, args ...interface{}) {
	logger.logf(LogLevelDebug, format, args...)
}

func (sink textSink) Write(entry LogEntry) error {
	_, err := fmt.Fprintf(sink.writer, "%s [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Level.String(), entry.Message)
	return err
}

func (sink *jsonFileSink) Write(entry LogEntry) error {
	record := jsonLogRecord{
		SchemaVersion: entry.SchemaVersion,
		Timestamp:     entry.Timestamp.Format(time.RFC3339Nano),
		Level:         strings.ToLower(entry.Level.String()),
		Message:       entry.Message,
		Event:         entry.Event,
		Fields:        entry.Fields,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if _, err := sink.file.Write(payload); err != nil {
		return err
	}
	_, err = sink.file.Write([]byte("\n"))
	return err
}

func parseEventMessage(message string) (string, map[string]any, bool) {
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, "event=") {
		return "", nil, false
	}

	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return "", nil, false
	}

	eventParts := strings.SplitN(tokens[0], "=", 2)
	if len(eventParts) != 2 || strings.TrimSpace(eventParts[1]) == "" {
		return "", nil, false
	}

	fields := map[string]any{}
	for _, token := range tokens[1:] {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		fields[key] = coerceFieldValue(parts[1])
	}

	return eventParts[1], fields, true
}

func coerceFieldValue(raw string) any {
	if parsedInt, err := strconv.Atoi(raw); err == nil {
		return parsedInt
	}
	if parsedBool, err := strconv.ParseBool(raw); err == nil {
		return parsedBool
	}
	return raw
}
