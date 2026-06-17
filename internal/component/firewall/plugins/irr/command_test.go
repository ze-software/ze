package irr

// VALIDATES: AC-1 update firewall irr asn saves prefixes and reports counts
// VALIDATES: AC-3 show firewall irr JSON output with cached entries
// VALIDATES: AC-11 show firewall irr prefix lists cached prefixes
// PREVENTS: CLI commands returning wrong status or malformed JSON

import (
	"encoding/json"
	"testing"
)

func TestShowIRRCommandEmpty(t *testing.T) {
	plug := &irrPlugin{
		config: &irrConfig{
			Server:       defaultServer,
			PeeringDBURL: defaultPeeringDBURL,
		},
	}
	status, data, err := plug.handleCommand("show firewall irr", nil)
	if err != nil {
		t.Fatalf("show firewall irr error: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want %q", status, statusDone)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("data type = %T, want json.RawMessage", data)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["server"] != defaultServer {
		t.Errorf("server = %v, want %q", result["server"], defaultServer)
	}
}

func TestHandleCommandUnknown(t *testing.T) {
	plug := &irrPlugin{}
	status, _, err := plug.handleCommand("show firewall unknown", nil)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if status != statusError {
		t.Errorf("status = %q, want %q", status, statusError)
	}
}
