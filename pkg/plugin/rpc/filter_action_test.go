package rpc

import (
	"encoding/json"
	"testing"
)

// TestFilterActionWireWithdraw proves "withdraw" is a filter verdict on the
// wire: a reply carrying it decodes to FilterWithdraw, and FilterWithdraw
// encodes back to the same word. Method: round-trip a FilterUpdateOutput
// through encoding/json, as a forked plugin's answer travels.
func TestFilterActionWireWithdraw(t *testing.T) {
	var out FilterUpdateOutput
	if err := json.Unmarshal([]byte(`{"action":"withdraw"}`), &out); err != nil {
		t.Fatalf("a withdraw verdict does not decode: %v", err)
	}
	if out.Action != FilterWithdraw {
		t.Fatalf("decoded action = %v, want withdraw", out.Action)
	}
	text, err := FilterWithdraw.MarshalText()
	if err != nil {
		t.Fatalf("FilterWithdraw does not encode: %v", err)
	}
	if string(text) != "withdraw" {
		t.Fatalf("FilterWithdraw encodes as %q, want \"withdraw\"", text)
	}
}
