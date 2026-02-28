package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

func TestDefaultSessionID(t *testing.T) {
	got := DefaultSessionID(mustParseTime(t, "2026-02-28T12:34:56Z"))
	if got != "20260228-123456" {
		t.Fatalf("expected formatted session id, got %q", got)
	}
}

func TestLoad_EmptySessionIDReturnsEmptyHistory(t *testing.T) {
	store := NewJSONFileStoreWithDir(t.TempDir())
	history, err := store.Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %d entries", len(history))
	}
}

func TestLoad_MissingSessionFileReturnsEmptyHistory(t *testing.T) {
	store := NewJSONFileStoreWithDir(t.TempDir())
	history, err := store.Load("missing")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %d entries", len(history))
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONFileStoreWithDir(dir)
	original := sampleHistory()

	if err := store.Save("roundtrip", original); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := store.Load("roundtrip")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(loaded) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(loaded))
	}
	if loaded[0].Role != llm.RoleSystem || loaded[0].Content != "system prompt" {
		t.Fatalf("unexpected system message after roundtrip")
	}
	if loaded[1].Role != llm.RoleUser || loaded[1].Content != "user prompt" {
		t.Fatalf("unexpected user message after roundtrip")
	}
	if loaded[2].Role != llm.RoleAssistant || len(loaded[2].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call preserved")
	}
	if loaded[2].ToolCalls[0].Name != "ListDir" {
		t.Fatalf("expected assistant tool call name preserved")
	}
	if loaded[3].Role != llm.RoleTool || loaded[3].ToolCallID != "call_1" {
		t.Fatalf("expected tool message preserved")
	}
	if loaded[4].Role != llm.RoleAssistant || loaded[4].Content != "done" {
		t.Fatalf("expected final assistant text preserved")
	}

	if _, err := os.Stat(filepath.Join(dir, "roundtrip.json")); err != nil {
		t.Fatalf("expected persisted session file: %v", err)
	}
}

func TestSave_EmptySessionIDNoop(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONFileStoreWithDir(dir)
	if err := store.Save("", sampleHistory()); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files written for empty session id")
	}
}

func TestLoad_InvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONFileStoreWithDir(dir)

	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := store.Load("broken")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoad_RejectsInvalidSessionID(t *testing.T) {
	store := NewJSONFileStoreWithDir(t.TempDir())
	if _, err := store.Load("../escape"); err == nil {
		t.Fatal("expected invalid session id error")
	}
}

func TestSave_RejectsInvalidSessionID(t *testing.T) {
	store := NewJSONFileStoreWithDir(t.TempDir())
	if err := store.Save("../../escape", sampleHistory()); err == nil {
		t.Fatal("expected invalid session id error")
	}
}

func TestSave_UsesRestrictivePermissions(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "nested", "sessions")
	store := NewJSONFileStoreWithDir(sessionsDir)

	if err := store.Save("secure", sampleHistory()); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	sessionFilePath := filepath.Join(sessionsDir, "secure.json")
	fileInfo, err := os.Stat(sessionFilePath)
	if err != nil {
		t.Fatalf("expected session file to exist: %v", err)
	}
	if fileInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected session file to not be group/world-accessible, got mode %o", fileInfo.Mode().Perm())
	}

	dirInfo, err := os.Stat(sessionsDir)
	if err != nil {
		t.Fatalf("expected sessions dir to exist: %v", err)
	}
	if dirInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected sessions dir to not be group/world-accessible, got mode %o", dirInfo.Mode().Perm())
	}
}

func sampleHistory() []llm.Message {
	return []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: "user prompt"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "ListDir", Arguments: "{}"}}},
		{Role: llm.RoleTool, ToolCallID: "call_1", Content: `{"entries":[]}`},
		{Role: llm.RoleAssistant, Content: "done"},
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time parse failed: %v", err)
	}
	return parsed
}
