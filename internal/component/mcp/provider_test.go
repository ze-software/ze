package mcp

import (
	"encoding/json"
	"net/http"
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
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": "ok"}},
	}
}

// TestStreamableProviderServesToolsAndCalls verifies the Provider path used by
// ze-chaos: server/discover reports the provider's server name, and
// tools/list + tools/call serve the provider's surface. Every POST is
// self-contained -- headers plus `_meta`, no handshake and no session id, which
// is exactly what the chaos .ci http=post checks can express.
// VALIDATES: spec-followup-subsystem AC-9 (chaos migration to NewStreamable),
// carried onto the 2026-07-28 transport.
func TestStreamableProviderServesToolsAndCalls(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Provider: fakeProvider{}})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodServerDiscover, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("server/discover status = %d, body=%v", status, parsed)
	}
	meta, _ := resultOf(t, parsed)[metaKey].(map[string]any)
	info, _ := meta[metaKeyServerInfo].(map[string]any)
	wantName := fakeProvider{}.ServerName()
	if info["name"] != wantName {
		t.Fatalf("serverInfo.name = %v, want %q", info["name"], wantName)
	}

	status, parsed = postMCP(t, hs, methodToolsList, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("tools/list status = %d, body=%v", status, parsed)
	}
	tools, _ := resultOf(t, parsed)["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("tools/list returned no provider tools: %v", parsed)
	}
	first, _ := tools[0].(map[string]any)
	if first["name"] != "chaos_status" {
		t.Fatalf("tools/list first tool = %v, want chaos_status", first["name"])
	}

	status, parsed = postMCP(t, hs, methodToolsCall, capsNone,
		`{"name":"chaos_status","arguments":{}}`)
	if status != http.StatusOK {
		t.Fatalf("tools/call status = %d, body=%v", status, parsed)
	}
	content, _ := resultOf(t, parsed)["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("tools/call returned no content: %v", parsed)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if text != "ok" {
		t.Fatalf("tools/call result = %q, want the fake provider payload", text)
	}
}

// TestStreamableProviderUnknownTool verifies a provider returning nil maps to
// the JSON-RPC invalid-params error, matching the legacy handler contract.
// VALIDATES: AC-9 provider error contract.
func TestStreamableProviderUnknownTool(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Provider: fakeProvider{}})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodToolsCall, capsNone,
		`{"name":"nope","arguments":{}}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body=%v", status, parsed)
	}
	rpcErr := rpcErrorOf(t, parsed)
	if rpcErr["code"] != float64(rpcInvalidParams) {
		t.Fatalf("code = %v, want %d", rpcErr["code"], rpcInvalidParams)
	}
	if msg, _ := rpcErr["message"].(string); !strings.Contains(msg, "unknown tool") {
		t.Fatalf("message = %q, want to name the unknown tool", msg)
	}
}

// TestStreamableHeaderValidationAppliesToProviderMode is the inversion of the
// pre-cutover TestStreamableWithoutProviderStillRequiresSession, which asserted
// a session-less POST was refused for the ze daemon and accepted for a Provider.
//
// VALIDATES: the session distinction is gone in BOTH directions -- neither mode
// requires a session id, and both modes require the standard headers.
// PREVENTS: Provider mode regaining a relaxed request contract of any kind.
func TestStreamableHeaderValidationAppliesToProviderMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  StreamableConfig
	}{
		{"provider mode", StreamableConfig{Provider: fakeProvider{}}},
		{"registry mode", StreamableConfig{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hs, cleanup := newTestStreamable(t, tc.cfg)
			defer cleanup()

			// Header-less POST: refused identically in both modes.
			status, parsed := postRaw(t, hs, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, nil)
			if status != http.StatusBadRequest {
				t.Fatalf("header-less POST: status = %d, want 400 (body %v)", status, parsed)
			}
			if code := rpcErrorOf(t, parsed)["code"]; code != float64(rpcHeaderMismatch) {
				t.Fatalf("header-less POST: code = %v, want %d", code, rpcHeaderMismatch)
			}

			// Conformant POST with no session id: served in both modes.
			if status, parsed := postMCP(t, hs, methodToolsList, capsNone, ""); status != http.StatusOK {
				t.Fatalf("conformant POST: status = %d, want 200 (body %v)", status, parsed)
			}
		})
	}
}
