package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

func TestDefaultRegistry_AllToolsReturnEnvelopeSchema(t *testing.T) {
	t.Setenv("SHIMIBOT_BASH_ALLOWLIST", "")
	t.Setenv("SHIMIBOT_BASH_DENYLIST", "")

	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "read-source.txt"), []byte("read me"), 0644); err != nil {
		t.Fatalf("failed preparing read file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "edit-target.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed preparing edit file: %v", err)
	}

	registry := DefaultRegistry()
	toolContext := ToolContext{
		CWD:         workspaceRoot,
		AllowedRoot: workspaceRoot,
		Timeout:     2 * time.Second,
	}

	testCases := []struct {
		toolName string
		args     map[string]any
	}{
		{toolName: "Bash", args: map[string]any{"command": "echo envelope_ok"}},
		{toolName: "EditPatch", args: map[string]any{"file_path": "edit-target.txt", "old_string": "hello", "new_string": "hola"}},
		{toolName: "Read", args: map[string]any{"file_path": "read-source.txt"}},
		{toolName: "Write", args: map[string]any{"file_path": "write-target.txt", "content": "written"}},
		{toolName: "ListDir", args: map[string]any{"path": "."}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.toolName, func(t *testing.T) {
			payload, err := json.Marshal(testCase.args)
			if err != nil {
				t.Fatalf("failed marshaling args: %v", err)
			}

			output, matched := registry.Execute(llm.ToolCall{Name: testCase.toolName, Arguments: string(payload)}, toolContext)
			if !matched {
				t.Fatalf("expected tool %s to be matched", testCase.toolName)
			}

			var envelope ResponseEnvelope
			if err := json.Unmarshal([]byte(output), &envelope); err != nil {
				t.Fatalf("expected valid envelope json, got: %v", err)
			}
			if !envelope.OK {
				t.Fatalf("expected ok envelope for tool %s, got %+v", testCase.toolName, envelope)
			}
			if envelope.Error != nil {
				t.Fatalf("expected no error for tool %s, got %+v", testCase.toolName, envelope.Error)
			}
			if envelope.Data == nil {
				t.Fatalf("expected data in envelope for tool %s", testCase.toolName)
			}
			if envelope.Meta == nil {
				t.Fatalf("expected meta in envelope for tool %s", testCase.toolName)
			}
			if envelope.Meta["tool"] != testCase.toolName {
				t.Fatalf("expected meta.tool=%s, got %v", testCase.toolName, envelope.Meta["tool"])
			}
		})
	}
}
