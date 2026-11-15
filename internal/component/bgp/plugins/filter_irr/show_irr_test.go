package filter_irr

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// VALIDATES: showIRR escapes the server a stored copy names, so a server value
// holding a quote cannot inject extra JSON keys into `show bgp irr` output.
// (Restores the test of the same name against the answer that reports the
// server per entry, from the pointer of the enforced copy.)
// METHOD: one enrolled ASN enforcing a list whose pointer names a server
// carrying a quote; parse the answer and look for the injected key.
func TestShowIRRServerEscaped(t *testing.T) {
	const server = `whois.example","injected":"x`
	st := newASNState(65001, "AS-TEST")
	st.list = &irrPrefixList{entries: mustPrefixListFromIRR(t, irr.PrefixList{
		IPv4: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
	})}
	st.pointer = store.Pointer{Server: server, Generation: 1}
	plug := &irrPlugin{byASN: map[uint32]*asnState{65001: st}}

	_, data, err := plug.showIRR()
	if err != nil {
		t.Fatalf("showIRR: %v", err)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("data is %T, want json.RawMessage", data)
	}
	var parsed struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("output is not valid JSON (injection): %v", err)
	}
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("output is not a JSON object: %v", err)
	}
	if _, injected := top["injected"]; injected {
		t.Fatal("the server value injected a top-level JSON key; not escaped")
	}
	if len(parsed.Entries) != 1 {
		t.Fatalf("entries = %v, want one", parsed.Entries)
	}
	if _, injected := parsed.Entries[0]["injected"]; injected {
		t.Error("the server value injected an extra JSON key into the entry; not escaped")
	}
	if parsed.Entries[0]["server"] != server {
		t.Errorf("server = %v, want the raw value preserved as one string", parsed.Entries[0]["server"])
	}
}
