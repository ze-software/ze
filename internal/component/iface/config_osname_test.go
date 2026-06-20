package iface

import "testing"

// VALIDATES: spec-iface-resolve-2 -- the os-name selector parses into the
// runtime config and osNameMap builds the logical->os override map the resolver
// consumes, skipping identity and absent selectors so every name == os-name
// config stays a no-op (backward compatibility, A-3).
// PREVENTS: a regression where the os-name leaf is parsed by nothing (the
// dormant-leaf state before this unit) or every ethernet entry produces an
// override (which would break the default-to-name resolution path).

// TestParseIfaceEntryOSName verifies the os-name leaf is read into ifaceEntry.
func TestParseIfaceEntryOSName(t *testing.T) {
	e, err := parseIfaceEntry("uplink", map[string]any{"os-name": "eth0"})
	if err != nil {
		t.Fatalf("parseIfaceEntry: %v", err)
	}
	if e.OSName != "eth0" {
		t.Errorf("OSName = %q, want eth0", e.OSName)
	}

	// Absent os-name leaves the field empty (default-to-name at resolve time).
	e2, err := parseIfaceEntry("eth1", map[string]any{})
	if err != nil {
		t.Fatalf("parseIfaceEntry (no os-name): %v", err)
	}
	if e2.OSName != "" {
		t.Errorf("OSName = %q, want empty", e2.OSName)
	}
}

// TestOSNameMapSkipsIdentityAndAbsent verifies osNameMap only emits real
// overrides, so identity (os-name == name) and absent selectors resolve to the
// logical name unchanged.
func TestOSNameMapSkipsIdentityAndAbsent(t *testing.T) {
	cfg := &ifaceConfig{Ethernet: []ifaceEntry{
		{Name: "uplink", OSName: "eth0"}, // real override
		{Name: "eth1", OSName: "eth1"},   // identity -> skipped
		{Name: "eth2"},                   // no override -> skipped
	}}
	m := cfg.osNameMap()
	if got := m["uplink"]; got != "eth0" {
		t.Errorf("uplink -> %q, want eth0", got)
	}
	if _, ok := m["eth1"]; ok {
		t.Error("identity os-name (eth1==eth1) must be skipped")
	}
	if _, ok := m["eth2"]; ok {
		t.Error("absent os-name must be skipped")
	}
	if len(m) != 1 {
		t.Errorf("override map size = %d, want 1", len(m))
	}
}
