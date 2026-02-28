package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type ToolContext struct {
	CWD         string
	AllowedRoot string
	Timeout     time.Duration
	Context     context.Context
	CorrelationID string
	Logger      Logger
}

type ResponseEnvelope struct {
	OK    bool           `json:"ok"`
	Data  any            `json:"data,omitempty"`
	Error *ResponseError `json:"error,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type ResponseError struct {
	Message string `json:"message"`
}

func ResolvePath(ctx ToolContext, pathValue string) string {
	trimmedPath := strings.TrimSpace(pathValue)
	if trimmedPath == "" {
		trimmedPath = "."
	}
	if filepath.IsAbs(trimmedPath) {
		return trimmedPath
	}

	base := strings.TrimSpace(ctx.CWD)
	if base == "" {
		base = "."
	}
	return filepath.Join(base, trimmedPath)
}

func EnsurePathAllowed(ctx ToolContext, resolvedPath string) error {
	allowedRoot := strings.TrimSpace(ctx.AllowedRoot)
	if allowedRoot == "" {
		return fmt.Errorf("invalid tool context: allowed_root is required")
	}

	absRoot, err := filepath.Abs(allowedRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve allowed_root: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("failed to evaluate allowed_root symlinks: %w", err)
	}

	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	realPath, err := resolveRealPath(absPath)
	if err != nil {
		return fmt.Errorf("failed to evaluate path symlinks: %w", err)
	}

	relPath, err := filepath.Rel(realRoot, realPath)
	if err != nil {
		return fmt.Errorf("failed to evaluate path policy: %w", err)
	}

	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path is outside allowed_root")
	}

	return nil
}

func resolveRealPath(absPath string) (string, error) {
	realPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return realPath, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	existingParent, err := findExistingParent(absPath)
	if err != nil {
		return "", err
	}
	realParent, err := filepath.EvalSymlinks(existingParent)
	if err != nil {
		return "", err
	}
	remainder, err := filepath.Rel(existingParent, absPath)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(realParent, remainder)), nil
}

func findExistingParent(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		if _, err := os.Lstat(current); err == nil {
			return current, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		next := filepath.Dir(current)
		if next == current {
			return "", fmt.Errorf("no existing parent found for path")
		}
		current = next
	}
}

func EffectiveTimeout(ctx ToolContext, fallback time.Duration) time.Duration {
	if ctx.Timeout > 0 {
		return ctx.Timeout
	}
	return fallback
}

func BaseContext(ctx ToolContext) context.Context {
	if ctx.Context != nil {
		return ctx.Context
	}
	return context.Background()
}

func SuccessEnvelope(data any, meta map[string]any) string {
	return encodeEnvelope(ResponseEnvelope{OK: true, Data: data, Meta: meta})
}

func ErrorEnvelope(message string, meta map[string]any) string {
	return encodeEnvelope(ResponseEnvelope{OK: false, Error: &ResponseError{Message: message}, Meta: meta})
}

func encodeEnvelope(envelope ResponseEnvelope) string {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return `{"ok":false,"error":{"message":"failed to encode tool response envelope"}}`
	}
	return string(payload)
}
