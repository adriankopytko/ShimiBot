package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

func TestBashTool_CommandPolicyBlocked(t *testing.T) {
	tool := BashTool{}
	_, err := tool.Execute(ToolContext{CWD: t.TempDir(), AllowedRoot: t.TempDir()}, `{"command":"rm -rf /"}`)
	if err == nil {
		t.Fatal("expected blocked command policy error")
	}
}

func TestBashTool_DenylistBlocksCommand(t *testing.T) {
	t.Setenv("SHIMIBOT_BASH_DENYLIST", `(?i)echo\s+hello`)
	tool := BashTool{}
	_, err := tool.Execute(ToolContext{CWD: t.TempDir(), AllowedRoot: t.TempDir()}, `{"command":"echo hello"}`)
	if err == nil {
		t.Fatal("expected denylist policy to block command")
	}
}

func TestBashTool_AllowlistRequiresMatch(t *testing.T) {
	t.Setenv("SHIMIBOT_BASH_ALLOWLIST", `(?i)^echo\b`)
	tool := BashTool{}

	if _, err := tool.Execute(ToolContext{CWD: t.TempDir(), AllowedRoot: t.TempDir()}, `{"command":"echo ok"}`); err != nil {
		t.Fatalf("expected allowlisted command to pass, got: %v", err)
	}

	if _, err := tool.Execute(ToolContext{CWD: t.TempDir(), AllowedRoot: t.TempDir()}, `{"command":"pwd"}`); err == nil {
		t.Fatal("expected non-allowlisted command to be blocked")
	}
}

func TestBashTool_InvalidPolicyRegexReturnsError(t *testing.T) {
	t.Setenv("SHIMIBOT_BASH_DENYLIST", `([`)
	tool := BashTool{}
	_, err := tool.Execute(ToolContext{CWD: t.TempDir(), AllowedRoot: t.TempDir()}, `{"command":"echo hi"}`)
	if err == nil {
		t.Fatal("expected invalid denylist regex to return error")
	}
}

func TestBashTool_Timeout(t *testing.T) {
	dir := t.TempDir()
	tool := BashTool{}
	_, err := tool.Execute(ToolContext{CWD: dir, AllowedRoot: dir, Timeout: 50 * time.Millisecond}, `{"command":"sleep 1"}`)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestReadTool_PathPolicyViolation(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "outside-file.txt")

	tool := ReadTool{}
	_, err := tool.Execute(ToolContext{CWD: root, AllowedRoot: root}, `{"file_path":"`+outside+`"}`)
	if err == nil {
		t.Fatal("expected path policy violation")
	}
}

func TestEnsurePathAllowed_BlocksExistingSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to prepare outside file: %v", err)
	}

	linkPath := filepath.Join(root, "link-out")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	ctx := ToolContext{CWD: root, AllowedRoot: root}
	err := EnsurePathAllowed(ctx, filepath.Join(linkPath, "secret.txt"))
	if err == nil {
		t.Fatal("expected symlink escape to be blocked")
	}
}

func TestEnsurePathAllowed_BlocksNonExistingSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	linkPath := filepath.Join(root, "link-out")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	ctx := ToolContext{CWD: root, AllowedRoot: root}
	err := EnsurePathAllowed(ctx, filepath.Join(linkPath, "new-file.txt"))
	if err == nil {
		t.Fatal("expected non-existing path under escaped symlink to be blocked")
	}
}

func TestEnsurePathAllowed_AllowsInRootSymlink(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "nested")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "ok.txt"), []byte("ok"), 0644); err != nil {
		t.Fatalf("failed to write nested file: %v", err)
	}

	linkPath := filepath.Join(root, "link-in")
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	ctx := ToolContext{CWD: root, AllowedRoot: root}
	err := EnsurePathAllowed(ctx, filepath.Join(linkPath, "ok.txt"))
	if err != nil {
		t.Fatalf("expected in-root symlink to be allowed, got: %v", err)
	}
}

func TestRegistryExecute_ContextCancelled(t *testing.T) {
	stub := &fakeTool{name: "Fake", result: "ok"}
	registry := NewRegistry(stub)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	output, matched := registry.Execute(llm.ToolCall{Name: "Fake", Arguments: "{}"}, ToolContext{
		CWD:         "/tmp",
		AllowedRoot: "/tmp",
		Context:     cancelledCtx,
	})
	if !matched {
		t.Fatal("expected matched tool")
	}
	if stub.executeInvoked != 0 {
		t.Fatalf("expected no tool execution when context is cancelled")
	}

	var envelope ResponseEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected valid envelope json: %v", err)
	}
	if envelope.OK {
		t.Fatalf("expected error envelope for cancelled context")
	}
	if envelope.Error == nil || envelope.Error.Message == "" {
		t.Fatalf("expected error details in envelope")
	}
}

func TestEnsureOutboundURLAllowed_BlocksLocalhost(t *testing.T) {
	err := EnsureOutboundURLAllowed(context.Background(), "http://localhost:8080/")
	if err == nil {
		t.Fatal("expected localhost URL to be blocked")
	}
}

func TestEnsureOutboundURLAllowed_BlocksPrivateIPv4(t *testing.T) {
	err := EnsureOutboundURLAllowed(context.Background(), "http://10.0.0.1/")
	if err == nil {
		t.Fatal("expected private IPv4 URL to be blocked")
	}
}

func TestEnsureOutboundURLAllowed_OverrideAllowsPrivateEgress(t *testing.T) {
	t.Setenv("SHIMIBOT_ALLOW_PRIVATE_EGRESS", "true")
	err := EnsureOutboundURLAllowed(context.Background(), "http://127.0.0.1:8080/")
	if err != nil {
		t.Fatalf("expected override to allow private egress, got: %v", err)
	}
}
