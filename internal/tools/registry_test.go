package tools

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type fakeTool struct {
	name           string
	capturedCtx    ToolContext
	capturedArgs   string
	result         any
	err            error
	executeInvoked int
}

func (tool *fakeTool) Name() string {
	return tool.name
}

func (tool *fakeTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{Name: tool.name}
}

func (tool *fakeTool) Execute(ctx ToolContext, arguments string) (any, error) {
	tool.executeInvoked++
	tool.capturedCtx = ctx
	tool.capturedArgs = arguments
	if tool.err != nil {
		return nil, tool.err
	}
	return tool.result, nil
}

func TestRegistryExecute_UsesEnvelopeAndContext(t *testing.T) {
	stub := &fakeTool{name: "Fake", result: map[string]any{"value": "ok"}}
	registry := NewRegistry(stub)
	ctx := ToolContext{CWD: "/tmp", AllowedRoot: "/tmp", Timeout: 2 * time.Second}

	output, matched := registry.Execute(llm.ToolCall{Name: "Fake", Arguments: `{"x":1}`}, ctx)
	if !matched {
		t.Fatal("expected matched tool")
	}
	if stub.executeInvoked != 1 {
		t.Fatalf("expected one tool execution, got %d", stub.executeInvoked)
	}
	if stub.capturedCtx.CWD != "/tmp" || stub.capturedCtx.AllowedRoot != "/tmp" {
		t.Fatalf("expected context propagated to tool")
	}
	if stub.capturedArgs != `{"x":1}` {
		t.Fatalf("expected normalized args passed to tool, got %q", stub.capturedArgs)
	}

	var envelope ResponseEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected valid envelope json, got error: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope, got %+v", envelope)
	}
	if envelope.Meta["tool"] != "Fake" {
		t.Fatalf("expected tool meta Fake, got %v", envelope.Meta["tool"])
	}
}

func TestRegistryExecute_ErrorEnvelopeOnToolFailure(t *testing.T) {
	stub := &fakeTool{name: "Fake", err: errors.New("boom")}
	registry := NewRegistry(stub)
	ctx := ToolContext{CWD: "/tmp", AllowedRoot: "/tmp", Timeout: 1 * time.Second}

	output, matched := registry.Execute(llm.ToolCall{Name: "Fake", Arguments: `{}`}, ctx)
	if !matched {
		t.Fatal("expected matched tool")
	}

	var envelope ResponseEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected valid envelope json, got error: %v", err)
	}
	if envelope.OK {
		t.Fatalf("expected error envelope, got %+v", envelope)
	}
	if envelope.Error == nil {
		t.Fatalf("expected error payload in envelope")
	}
}

func TestRegistryExecute_ErrorEnvelopeOnInvalidContext(t *testing.T) {
	stub := &fakeTool{name: "Fake", result: "ok"}
	registry := NewRegistry(stub)

	output, matched := registry.Execute(llm.ToolCall{Name: "Fake", Arguments: `{}`}, ToolContext{})
	if !matched {
		t.Fatal("expected matched tool")
	}
	if stub.executeInvoked != 0 {
		t.Fatalf("expected tool not invoked on invalid context")
	}

	var envelope ResponseEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected valid envelope json, got error: %v", err)
	}
	if envelope.OK {
		t.Fatalf("expected error envelope for invalid context")
	}
}
