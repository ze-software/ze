package iface

import "testing"

// VALIDATES: the mac/match selector parses into ifaceEntry.MatchMAC and
// permMACMap builds the logical->match-MAC map the resolver consumes, scoped to
// ethernet (the matched physical kind) only. mac/address (override) and
// mac/match (selector) are independent.
// PREVENTS: a regression where the mac/match leaf is parsed by nothing, or a
// created kind (dummy/veth/bridge) produces a MAC selector it can never honor.

// TestParseIfaceEntryMatchMAC verifies the mac/match leaf is read into
// ifaceEntry, independently of the mac/address override.
func TestParseIfaceEntryMatchMAC(t *testing.T) {
	e, err := parseIfaceEntry("uplink", map[string]any{
		"mac": map[string]any{"match": "aa:bb:cc:dd:ee:ff"},
	})
	if err != nil {
		t.Fatalf("parseIfaceEntry: %v", err)
	}
	if e.MatchMAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MatchMAC = %q, want aa:bb:cc:dd:ee:ff", e.MatchMAC)
	}

	// Override and match are independent: both parse from the same mac container.
	e2, err := parseIfaceEntry("uplink", map[string]any{
		"mac": map[string]any{"address": "02:00:00:00:00:01", "match": "aa:bb:cc:dd:ee:ff"},
	})
	if err != nil {
		t.Fatalf("parseIfaceEntry both: %v", err)
	}
	if e2.MACAddress != "02:00:00:00:00:01" || e2.MatchMAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("override+match not both parsed: addr=%q match=%q", e2.MACAddress, e2.MatchMAC)
	}

	// Absent mac/match leaves the field empty (bind by name/os-name).
	e3, err := parseIfaceEntry("eth1", map[string]any{})
	if err != nil {
		t.Fatalf("parseIfaceEntry (no mac): %v", err)
	}
	if e3.MatchMAC != "" {
		t.Errorf("MatchMAC = %q, want empty", e3.MatchMAC)
	}
}

// TestPermMACMapEthernetOnly verifies permMACMap emits selectors only for
// ethernet entries, never for the Ze-created kinds (which are identified by the
// name Ze assigns).
func TestPermMACMapEthernetOnly(t *testing.T) {
	cfg := &ifaceConfig{
		Ethernet: []ifaceEntry{
			{Name: "uplink", MatchMAC: "aa:bb:cc:dd:ee:01"},
			{Name: "eth2"}, // no selector -> skipped
		},
		Dummy: []ifaceEntry{
			{Name: "dummy0", MatchMAC: "aa:bb:cc:dd:ee:02"}, // created kind -> ignored
		},
	}
	m := cfg.permMACMap()
	if got := m["uplink"]; got != "aa:bb:cc:dd:ee:01" {
		t.Errorf("uplink -> %q, want aa:bb:cc:dd:ee:01", got)
	}
	if _, ok := m["dummy0"]; ok {
		t.Error("created-kind mac/match must be ignored")
	}
	if _, ok := m["eth2"]; ok {
		t.Error("absent mac/match must be skipped")
	}
	if len(m) != 1 {
		t.Errorf("match map size = %d, want 1", len(m))
	}
}

// TestValidateUniqueMatchMAC verifies two ethernet interfaces cannot select the
// same device by MAC (case-insensitively), while distinct or absent selectors
// are accepted.
func TestValidateUniqueMatchMAC(t *testing.T) {
	ok := &ifaceConfig{Ethernet: []ifaceEntry{
		{Name: "uplink", MatchMAC: "aa:bb:cc:dd:ee:01"},
		{Name: "wan", MatchMAC: "aa:bb:cc:dd:ee:02"},
		{Name: "lan"}, // no selector
	}}
	if err := validateUniqueMatchMAC(ok); err != nil {
		t.Errorf("distinct match MACs must validate: %v", err)
	}

	dup := &ifaceConfig{Ethernet: []ifaceEntry{
		{Name: "uplink", MatchMAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "wan", MatchMAC: "AA:BB:CC:DD:EE:FF"}, // same MAC, different case
	}}
	if err := validateUniqueMatchMAC(dup); err == nil {
		t.Error("two interfaces matching the same MAC must be rejected")
	}
}
