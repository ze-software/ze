package mcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

// TestMalformedMetaRejected covers AC-16.
//
// VALIDATES: a request whose params._meta omits a REQUIRED field is rejected
// with HTTP 400 and JSON-RPC -32602 (Invalid params), while an omitted
// clientInfo -- which the specification makes a SHOULD, not a MUST -- is
// accepted.
// PREVENTS: collapsing "missing _meta field" into the -32020 header-mismatch
// code, which would emit a reserved-range code with a meaning the
// specification does not give it.
func TestMalformedMetaRejected(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	tests := []struct {
		name       string
		meta       string
		wantStatus int
		wantCode   float64
	}{
		{
			name:       "protocolVersion absent",
			meta:       `{"io.modelcontextprotocol/clientCapabilities":{}}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   -32602,
		},
		{
			name:       "clientCapabilities absent",
			meta:       `{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   -32602,
		},
		{
			name:       "_meta absent entirely",
			meta:       "",
			wantStatus: http.StatusBadRequest,
			wantCode:   -32602,
		},
		{
			name:       "clientCapabilities not an object",
			meta:       `{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `","io.modelcontextprotocol/clientCapabilities":"yes"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   -32602,
		},
		{
			name:       "clientInfo absent is accepted",
			meta:       `{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `","io.modelcontextprotocol/clientCapabilities":{}}`,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := "{}"
			if tt.meta != "" {
				params = `{"_meta":` + tt.meta + `}`
			}
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":` + params + `}`
			status, parsed := postRaw(t, hs, body, map[string]string{
				"MCP-Protocol-Version": ProtocolVersion,
				"Mcp-Method":           "tools/list",
			})
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %v)", status, tt.wantStatus, parsed)
			}
			if tt.wantCode == 0 {
				if _, isErr := parsed["error"]; isErr {
					t.Fatalf("unexpected error response: %v", parsed)
				}
				return
			}
			rpcErr, ok := parsed["error"].(map[string]any)
			if !ok {
				t.Fatalf("no error object in %v", parsed)
			}
			if rpcErr["code"] != tt.wantCode {
				t.Fatalf("code = %v, want %v", rpcErr["code"], tt.wantCode)
			}
		})
	}
}

// VALIDATES: parseRequestMeta returns the three distinct sentinel errors and,
// on success, the parsed version / info / capabilities triple.
// PREVENTS: a permissive fallback that would let a request with no declared
// version reach dispatch.
func TestParseRequestMeta(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		wantErr error
		want    requestMeta
	}{
		{
			name:    "no params object",
			params:  `[]`,
			wantErr: errMetaMissing,
		},
		{
			name:    "no _meta",
			params:  `{"name":"x"}`,
			wantErr: errMetaMissing,
		},
		{
			name:    "empty version string",
			params:  `{"_meta":{"io.modelcontextprotocol/protocolVersion":"","io.modelcontextprotocol/clientCapabilities":{}}}`,
			wantErr: errMetaProtocolVersion,
		},
		{
			name:    "capabilities null",
			params:  `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":null}}`,
			wantErr: errMetaClientCapabilities,
		},
		{
			// Neither member declares anything this server gates on.
			// `resources` is a ServerCapabilities member, not a
			// ClientCapabilities one. And the bare `tasks` member is the
			// 2025-11-25 core-protocol spelling. MCP 2026-07-28 moved tasks
			// onto the io.modelcontextprotocol/tasks EXTENSION, so task support
			// is declared under `extensions` and nowhere else. A server that
			// honored the stale spelling would push an unsolicited task handle
			// at a legacy client that never agreed to receive one (D-1).
			name:   "full, with two capabilities this server does not gate on",
			params: `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"c","version":"1.2"},"io.modelcontextprotocol/clientCapabilities":{"resources":{},"tasks":{}}}}`,
			want: requestMeta{
				ProtocolVersion: "2026-07-28",
				ClientInfo:      clientInfo{Name: "c", Version: "1.2"},
				Capabilities:    clientCapabilities{},
			},
		},
		{
			name:   "tasks via extension identifier",
			params: `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"extensions":{"io.modelcontextprotocol/tasks":{}}}}}`,
			want: requestMeta{
				ProtocolVersion: "2026-07-28",
				Capabilities:    clientCapabilities{Tasks: true},
			},
		},
		{
			name:   "non-object capability value is not a declaration",
			params: `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"resources":true,"tasks":"yes"}}}`,
			want: requestMeta{
				ProtocolVersion: "2026-07-28",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRequestMeta(decodeParamsObject(json.RawMessage(tt.params)))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.ProtocolVersion != tt.want.ProtocolVersion {
				t.Errorf("ProtocolVersion = %q, want %q", got.ProtocolVersion, tt.want.ProtocolVersion)
			}
			if got.ClientInfo != tt.want.ClientInfo {
				t.Errorf("ClientInfo = %+v, want %+v", got.ClientInfo, tt.want.ClientInfo)
			}
			if got.Capabilities != tt.want.Capabilities {
				t.Errorf("Capabilities = %+v, want %+v", got.Capabilities, tt.want.Capabilities)
			}
		})
	}
}

// test-relax: the `zero.Resources` assertion is replaced, not dropped. Its
// subject, the resources CLIENT-capability gate, was removed outright.
// `resources` is a ServerCapabilities member and no conformant client can
// declare it, so the gate refused every conformant caller. The replacement
// below asserts that the removal did not turn into a silent re-declaration
// elsewhere, and resources_test.go asserts the served behavior end to end.
//
// VALIDATES: the zero clientCapabilities denies every gated capability, and a
// client naming a SERVER capability declares nothing this server gates on.
// PREVENTS: reintroducing a capability shape whose zero value reads as
// "supported" (R-3, ai/rules/fail-closed-guards.md), and re-adding a
// client-side gate on a server capability under a new name.
func TestZeroClientCapabilitiesDenies(t *testing.T) {
	var zero clientCapabilities
	if zero.Tasks {
		t.Error("zero clientCapabilities.Tasks = true, want false")
	}
	serverCaps := parseClientCapabilities(map[string]any{
		"resources": map[string]any{},
		"tools":     map[string]any{},
		"prompts":   map[string]any{},
	})
	if serverCaps != zero {
		t.Errorf("parseClientCapabilities(server capabilities) = %+v, want the zero value %+v", serverCaps, zero)
	}
}
