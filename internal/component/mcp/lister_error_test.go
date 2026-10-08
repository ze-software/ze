package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// errListerProbe stands for the YANG loader refusing the schema, the error the
// hub's command lister returns when command metadata cannot be built.
var errListerProbe = errors.New("probe: undeclared YANG extension ze:hepl")

// TestCommandListerErrorIsReported: when the command lister returns an error,
// tools/list and a tools/call for a generated tool answer a JSON-RPC internal
// error naming the cause, instead of a tool list holding only the handcrafted
// tools or an "unknown tool" refusal. Method: hand a Streamable a lister that
// always fails and read the JSON-RPC error of each method.
//
// VALIDATES: the MCP surface reports why command metadata is missing.
// PREVENTS: a client seeing no generated tool and no reason.
func TestCommandListerErrorIsReported(t *testing.T) {
	s := &Streamable{cfg: StreamableConfig{
		Commands: func() ([]CommandInfo, error) { return nil, errListerProbe },
	}}

	if _, err := s.allTools(clientCapabilities{}); !errors.Is(err, errListerProbe) {
		t.Fatalf("allTools error = %v, want the lister error", err)
	}

	id := json.RawMessage(`1`)
	call, err := json.Marshal(callParams{Name: "ze_show_bgp", Arguments: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		req  request
	}{
		{"tools/list", request{JSONRPC: "2.0", ID: &id, Method: methodToolsList}},
		{"tools/call", request{JSONRPC: "2.0", ID: &id, Method: methodToolsCall, Params: call}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := s.dispatchMethod(t.Context(), requestScope{}, &tc.req, "198.51.100.7:4000")
			if resp == nil || resp.Error == nil {
				t.Fatalf("response %+v carries no JSON-RPC error", resp)
			}
			if resp.Error.Code != rpcInternalError {
				t.Errorf("error code = %d, want %d", resp.Error.Code, rpcInternalError)
			}
			if !strings.Contains(resp.Error.Message, "ze:hepl") {
				t.Errorf("error message %q does not carry the cause", resp.Error.Message)
			}
		})
	}
}
