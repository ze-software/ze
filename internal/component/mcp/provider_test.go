package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeProvider implements ToolProvider with a single custom tool, mirroring
// the ze-chaos chaosmcp.Provider shape.
type fakeProvider struct{}

func (fakeProvider) ServerName() string { return "fake-chaos-mcp" }

func (fakeProvider) Tools() []map[string]any {
	return []map[string]any{{
		"name":        "chaos_status",
		"description": "fake chaos status",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	}}
}

func (fakeProvider) CallTool(name string, _ json.RawMessage) map[string]any {
	if name != "chaos_status" {
		return nil
	}
	return TextResult(`{"routes-announced":42}`)
}

// postJSON posts a JSON-RPC body without any session header, mirroring the
// chaos .ci http=post checks (each POST independent, no Mcp-Session-Id).
func postJSON(t *testing.T, hs *httptest.Server, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer closeBody(t, resp.Body)
	var sb strings.Builder
	if _, err := io.Copy(&sb, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, sb.String()
}

// TestStreamableProviderServesSessionless verifies the Provider path used by
// ze-chaos after the legacy handler deletion: initialize reports the
// provider's server name, and tools/list + tools/call work WITHOUT an
// Mcp-Session-Id header (the chaos .ci http=post checks cannot thread one).
// VALIDATES: spec-followup-subsystem AC-9 (chaos migration to NewStreamable).
func TestStreamableProviderServesSessionless(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{Provider: fakeProvider{}})
	defer cleanup()

	status, body := postJSON(t, hs, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if status != http.StatusOK {
		t.Fatalf("initialize status = %d, body=%s", status, body)
	}
	if !strings.Contains(body, "fake-chaos-mcp") {
		t.Fatalf("initialize should report provider ServerName, got %s", body)
	}

	status, body = postJSON(t, hs, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if status != http.StatusOK {
		t.Fatalf("tools/list status = %d, body=%s", status, body)
	}
	if !strings.Contains(body, "chaos_status") {
		t.Fatalf("tools/list should contain provider tool, got %s", body)
	}

	status, body = postJSON(t, hs, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"chaos_status","arguments":{}}}`)
	if status != http.StatusOK {
		t.Fatalf("tools/call status = %d, body=%s", status, body)
	}
	if !strings.Contains(body, "routes-announced") {
		t.Fatalf("tools/call should return provider result, got %s", body)
	}
}

// TestStreamableProviderUnknownTool verifies a provider returning nil maps to
// the JSON-RPC invalid-params error, matching the legacy handler contract.
// VALIDATES: AC-9 provider error contract.
func TestStreamableProviderUnknownTool(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{Provider: fakeProvider{}})
	defer cleanup()

	status, body := postJSON(t, hs, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(body, "-32602") || !strings.Contains(body, "unknown tool") {
		t.Fatalf("expected -32602 unknown tool, got %s", body)
	}
}

// TestStreamableWithoutProviderStillRequiresSession pins ze's strict-session
// behavior: with no Provider configured, a session-less non-initialize POST is
// rejected, exactly as before AC-9.
// VALIDATES: AC-9 does not relax the ze daemon path.
func TestStreamableWithoutProviderStillRequiresSession(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, body := postJSON(t, hs, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(body, "Mcp-Session-Id header required") {
		t.Fatalf("expected session requirement error, got %s", body)
	}
}
