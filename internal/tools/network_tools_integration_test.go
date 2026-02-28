package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/adriankopytko/ShimiBot/internal/llm"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestRegistryExecute_FetchWebPage_WithMockedPublicHost(t *testing.T) {
	registry := DefaultRegistry()
	toolContext := ToolContext{CWD: t.TempDir(), AllowedRoot: t.TempDir(), Timeout: 2 * time.Second}

	originalResolve := resolveIPAddrs
	resolveIPAddrs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	defer func() {
		resolveIPAddrs = originalResolve
	}()

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "fetch.test" {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(bytes.NewBufferString("<html><body><h1>Hello</h1><p>world</p></body></html>")),
		}, nil
	})
	defer func() {
		http.DefaultTransport = originalTransport
	}()

	output, matched := registry.Execute(llm.ToolCall{Name: "FetchWebPage", Arguments: `{"url":"https://fetch.test/page"}`}, toolContext)
	if !matched {
		t.Fatal("expected FetchWebPage to be matched")
	}

	var envelope ResponseEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected valid envelope json, got: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope, got %+v", envelope)
	}
}

func TestRegistryExecute_WebSearchOllama_WithMockedPublicHost(t *testing.T) {
	t.Setenv("OLLAMA_WEB_SEARCH_URL", "https://search.test/query")
	t.Setenv("OLLAMA_WEB_SEARCH_API_KEY", "")

	registry := DefaultRegistry()
	toolContext := ToolContext{CWD: t.TempDir(), AllowedRoot: t.TempDir(), Timeout: 2 * time.Second}

	originalResolve := resolveIPAddrs
	resolveIPAddrs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	defer func() {
		resolveIPAddrs = originalResolve
	}()

	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "search.test" {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"results":[{"title":"Result","url":"https://example.com","snippet":"Example snippet"}]}`)),
		}, nil
	})
	defer func() {
		http.DefaultTransport = originalTransport
	}()

	output, matched := registry.Execute(llm.ToolCall{Name: "WebSearchOllama", Arguments: `{"query":"golang","max_results":1}`}, toolContext)
	if !matched {
		t.Fatal("expected WebSearchOllama to be matched")
	}

	var envelope ResponseEnvelope
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected valid envelope json, got: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope, got %+v", envelope)
	}
}
