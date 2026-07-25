package as112

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: AC-8 -- show as112 reports the same state the server reads.
func TestShowAS112_MatchesServerSnapshot(t *testing.T) {
	resetAS112State(t)
	storeState(buildState(as112Config{Enabled: true, Hostname: "node1", AddressFamily: addressFamilyBoth}, 7))

	resp, err := handleShowAS112(nil, nil)
	if err != nil {
		t.Fatalf("handleShowAS112: unexpected error: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("resp.Data is %T, want plugin.Map", resp.Data)
	}
	if data["enabled"] != true {
		t.Fatalf("enabled = %v, want true", data["enabled"])
	}
	if data["hostname"] != "node1" {
		t.Fatalf("hostname = %v, want node1", data["hostname"])
	}
	if data["soa-serial"] != uint32(7) {
		t.Fatalf("soa-serial = %v, want 7", data["soa-serial"])
	}
}

// VALIDATES: show as112 before any config is applied reports disabled, not a
// panic, AND still surfaces address-registry status -- a registry failure
// from another consumer must be visible even before as112 itself has ever
// been configured (fork round-2 finding 3: the pre-config early-return
// branch used to skip this block entirely).
func TestShowAS112_NoStateYet(t *testing.T) {
	resetAS112State(t)
	resp, err := handleShowAS112(nil, nil)
	if err != nil {
		t.Fatalf("handleShowAS112: unexpected error: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("resp.Data is %T, want plugin.Map", resp.Data)
	}
	if data["enabled"] != false {
		t.Fatalf("enabled = %v, want false", data["enabled"])
	}
	if _, present := data["address-registry-ok"]; !present {
		t.Fatal("address-registry-ok absent before as112 has been configured, want present")
	}
}

// VALIDATES: show as112 surfaces the address-registry's health so an
// operator can see a stuck registration failure without tailing logs --
// the registry-triggered reconcile trigger (iface.RegisterOwnedAddresses's
// async callback) is otherwise fire-and-forget with only a log line.
// PREVENTS: a silent, invisible address-registration failure (fork finding 2).
//
// Reads the REAL, un-mocked iface.RegistryReconcileStatus() -- package-level
// global state in a different package, with no reset hook exported for
// cross-package tests. Only safe because nothing else in this test binary
// calls the real iface.RegisterOwnedAddresses/UnregisterOwnedAddresses
// (this package's other tests all go through applyAddressRegistration's
// injectable registerFn/unregisterFn seam, e.g.
// TestOnConfigure_RegistersAddressesOnEnable). If a future test in this
// package starts calling the real iface functions, this assertion can flake.
func TestShowAS112_SurfacesAddressRegistryStatus(t *testing.T) {
	resetAS112State(t)
	storeState(buildState(as112Config{Enabled: true, AddressFamily: addressFamilyBoth}, 1))

	resp, err := handleShowAS112(nil, nil)
	if err != nil {
		t.Fatalf("handleShowAS112: unexpected error: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("resp.Data is %T, want plugin.Map", resp.Data)
	}
	if data["address-registry-ok"] != true {
		t.Fatalf("address-registry-ok = %v, want true (no registry-triggered reconcile has run in this test)", data["address-registry-ok"])
	}
	if _, present := data["address-registry-error"]; present {
		t.Fatalf("address-registry-error present = %v, want absent when ok", data["address-registry-error"])
	}
}
