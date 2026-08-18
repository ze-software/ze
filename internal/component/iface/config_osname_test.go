package iface

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
)

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

// TestOSDeviceNameValidatorScreensForm verifies the os-name leaf's validator
// refuses a value no kernel device can carry, and accepts one that is merely
// absent from this box: the YANG promises a binding that defers until its device
// appears, and a config is validated on machines that will never run it.
//
// VALIDATES: spec-fixit-iface-selector-ignored-by-apply, the editor surface.
// PREVENTS: `ze config validate` accepting an os-name the backend can only fail
// on, and rejecting an alias for hardware that is not plugged in yet.
func TestOSDeviceNameValidatorScreensForm(t *testing.T) {
	validate := config.OSDeviceNameValidator().ValidateFn

	for _, accepted := range []string{"eth0", "enp1s0f0np0", "a"} {
		if err := validate("interface/ethernet/wan/os-name", accepted); err != nil {
			t.Errorf("os-name %q refused: %v", accepted, err)
		}
	}
	for _, refused := range []string{"", "this-name-is-far-too-long", "eth/0", "../etc", "eth 0"} {
		if err := validate("interface/ethernet/wan/os-name", refused); err == nil {
			t.Errorf("os-name %q accepted; no kernel device can carry it", refused)
		}
	}
}

// TestOSDeviceNameCompletionOffersPresentDevices verifies the os-name selector
// completes to the devices this box carries. A mistyped alias is not refused by
// validation, so completion is what separates a typo from an intent.
//
// VALIDATES: spec-fixit-iface-selector-ignored-by-apply, the editor surface.
// PREVENTS: the os-name leaf staying a blind free-text field.
func TestOSDeviceNameCompletionOffersPresentDevices(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{
		"enp1s0": {name: "enp1s0", linkType: "device", index: 2},
		"enp2s0": {name: "enp2s0", linkType: "device", index: 3},
	}}
	const backendName = "os-device-name-completion"
	if err := RegisterBackend(backendName, func() (Backend, error) { return b, nil }); err != nil {
		t.Fatalf("register backend: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, backendName)
		backendsMu.Unlock()
	})
	if err := LoadBackend(backendName); err != nil {
		t.Fatalf("load backend: %v", err)
	}

	got := osDeviceNameCompleteFn()

	assert.ElementsMatch(t, []string{"enp1s0", "enp2s0"}, got)
}
