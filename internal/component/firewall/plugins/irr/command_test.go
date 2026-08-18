package irr

// VALIDATES: AC-1 update firewall irr asn saves prefixes and reports counts
// VALIDATES: AC-3 show firewall irr JSON output with cached entries
// VALIDATES: AC-11 show firewall irr prefix lists cached prefixes
// PREVENTS: CLI commands returning wrong status or malformed JSON

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
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

// showEntries runs "show firewall irr" and returns its entry list.
func showEntries(t *testing.T, plug *irrPlugin) []map[string]any {
	t.Helper()
	status, data, err := plug.handleCommand("show firewall irr", nil)
	if err != nil {
		t.Fatalf("show firewall irr: %v", err)
	}
	if status != statusDone {
		t.Fatalf("status = %q, want %q", status, statusDone)
	}
	raw, ok := data.(json.RawMessage)
	if !ok {
		t.Fatalf("data type = %T, want json.RawMessage", data)
	}
	var result struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return result.Entries
}

// VALIDATES: AC-5 -- after a refresh that learned nothing, the show command
// reports the entry as stale and gives the age of the data being enforced.
// PREVENTS: an operator reading "status ok" while the filter runs on prefixes
// the IRR has stopped confirming.
func TestShowIRRReportsStaleEntry(t *testing.T) {
	addr := fakeIRRWhois(t, map[string]string{
		"!a4AS-TEST": "A1\n10.0.0.0/24\nC\n",
	})
	plug := &irrPlugin{
		prefixStore: store.New(irr.NewIRR(addr), nil, ""),
		config: &irrConfig{
			Server: addr,
			refs:   []irrRef{{Name: "AS-TEST", IsASSet: true, TableName: "ze_wan"}},
		},
	}
	if _, err := plug.prefixStore.Refresh(context.Background(), "AS-TEST", "AS-TEST"); err != nil {
		t.Fatalf("seed refresh: %v", err)
	}

	entries := showEntries(t, plug)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0]["status"] != "ok" {
		t.Errorf("fresh entry status = %v, want ok", entries[0]["status"])
	}
	if _, reported := entries[0]["stale-since"]; reported {
		t.Error("a fresh entry must not report a stale-since time")
	}
	if _, reported := entries[0]["data-age-seconds"]; !reported {
		t.Error("show must report the age of the data being enforced")
	}

	// A second refresh against a server that answers "key not found".
	plug.prefixStore = store.New(irr.NewIRR(fakeIRRWhois(t, nil)), nil, "")
	plug.prefixStore.Put("AS-TEST", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}, nil)
	if _, err := plug.prefixStore.Refresh(context.Background(), "AS-TEST", "AS-TEST"); err == nil {
		t.Fatal("expected the empty answer to be reported")
	}

	entries = showEntries(t, plug)
	if entries[0]["status"] != "stale" {
		t.Errorf("status = %v, want stale", entries[0]["status"])
	}
	if _, reported := entries[0]["stale-since"]; !reported {
		t.Error("a stale entry must report since when")
	}
	if count, _ := entries[0]["ipv4-count"].(float64); count != 1 {
		t.Errorf("ipv4-count = %v, want 1: the prefixes must still be enforced", entries[0]["ipv4-count"])
	}
}

// VALIDATES: AC-6 -- clear firewall irr removes the cached prefixes, and says so
// when there is nothing to remove.
// PREVENTS: a deregistered AS-SET being enforced forever, now that an empty
// answer no longer clears it.
func TestClearFirewallIRRPurgesEntry(t *testing.T) {
	ps := store.New(nil, nil, "")
	ps.Put("AS-TEST", []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}, nil)
	plug := &irrPlugin{prefixStore: ps, config: &irrConfig{Server: defaultServer}}

	status, _, err := plug.handleCommand("clear firewall irr as-set", []string{"AS-TEST"})
	if err != nil {
		t.Fatalf("clear firewall irr as-set: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want %q", status, statusDone)
	}
	if got := ps.Get("AS-TEST"); got != nil {
		t.Fatalf("entry survived the clear: %+v", got)
	}

	if _, _, err := plug.handleCommand("clear firewall irr as-set", []string{"AS-TEST"}); err == nil {
		t.Error("clearing an absent entry must report that there was nothing to clear")
	}
	if _, _, err := plug.handleCommand("clear firewall irr as-set", nil); err == nil {
		t.Error("clear with no argument must report its usage")
	}
	if _, _, err := plug.handleCommand("clear firewall irr asn", []string{"0"}); err == nil {
		t.Error("clear firewall irr asn must reject ASN 0")
	}
}
