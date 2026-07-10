package ifacevpp

import (
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/lcp"

	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
)

func newLCPBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	b.names.Add("loop0", 5, "loop0")
	return b
}

// withLCPSettings overrides the active LCP settings seam (as if the VPP
// component were running with these settings) for the duration of a test.
func withLCPSettings(t *testing.T, settings vppcomp.LCPSettings) {
	t.Helper()
	prev := getActiveLCPSettings
	getActiveLCPSettings = func() (vppcomp.LCPSettings, bool) { return settings, true }
	t.Cleanup(func() { getActiveLCPSettings = prev })
}

// TestSetupLCPPairCreate verifies AC-6: with LCP enabled, SetupLCPPair issues
// lcp_itf_pair_add_del (add) with the resolved SwIfIndex, the host TAP name, and
// TAP host type. A root-reachable netns (host) maps to the empty per-pair netns.
// VALIDATES: AC-6 -- LCP pair created via LcpItfPairAddDel.
// PREVENTS: regression to a missing / no-op LCP path.
func TestSetupLCPPairCreate(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)

	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	req, ok := ch.requests[0].(*lcp.LcpItfPairAddDel)
	if !ok {
		t.Fatalf("request type: got %T, want *lcp.LcpItfPairAddDel", ch.requests[0])
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.SwIfIndex != 5 {
		t.Errorf("SwIfIndex: got %d, want 5", req.SwIfIndex)
	}
	if req.HostIfName != "loop0" {
		t.Errorf("HostIfName: got %q, want loop0", req.HostIfName)
	}
	if req.HostIfType != lcp.LCP_API_ITF_HOST_TAP {
		t.Errorf("HostIfType: got %v, want TAP", req.HostIfType)
	}
	if req.Netns != "" {
		t.Errorf("Netns: got %q, want \"\" (host maps to host netns)", req.Netns)
	}
}

// TestSetupLCPPairNetnsPassthrough verifies a non-root netns is passed to VPP
// verbatim (so the operator can isolate the TAP), which the doctor check warns
// about when BGP is enabled.
func TestSetupLCPPairNetnsPassthrough(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", ""); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	req, ok := ch.requests[0].(*lcp.LcpItfPairAddDel)
	if !ok {
		t.Fatalf("request type: got %T", ch.requests[0])
	}
	if req.Netns != "dataplane" {
		t.Errorf("Netns: got %q, want dataplane", req.Netns)
	}
	if req.HostIfName != "loop0" {
		t.Errorf("HostIfName default: got %q, want loop0 (defaults to ze name)", req.HostIfName)
	}
}

// TestSetupLCPPairDisabledNoop verifies SetupLCPPair is a no-op when LCP is
// disabled, so config-apply can call it unconditionally for vpp loopbacks.
func TestSetupLCPPairDisabledNoop(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: false})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("LCP disabled: expected no VPP request, got %d", len(ch.requests))
	}
}

// TestSetupLCPPairNameTooLong verifies AC-7: a host name over the 15-byte Linux
// limit is rejected (no silent truncation).
func TestSetupLCPPairNameTooLong(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	b.names.Add("sixteencharacter", 6, "sixteencharacter")
	err := b.SetupLCPPair("sixteencharacter", "sixteencharacter") // 16 chars
	if err == nil {
		t.Fatal("expected error for host name > 15 bytes, got nil")
	}
	if !strings.Contains(err.Error(), "IFNAMSIZ") {
		t.Errorf("expected IFNAMSIZ in error, got: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("expected no VPP request on rejection, got %d", len(ch.requests))
	}
}

// TestSetupLCPPairCollision verifies R-5: two ze interfaces mapping to the same
// host TAP name is rejected before it collides in VPP.
func TestSetupLCPPairCollision(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	b.names.Add("loop1", 7, "loop1")

	if err := b.SetupLCPPair("loop0", "shadow0"); err != nil {
		t.Fatalf("first SetupLCPPair: %v", err)
	}
	err := b.SetupLCPPair("loop1", "shadow0")
	if err == nil {
		t.Fatal("expected collision error for duplicate host name, got nil")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("expected 'already used' in error, got: %v", err)
	}
}

// TestRemoveLCPPair verifies RemoveLCPPair issues lcp_itf_pair_add_del (del) for
// a recorded pair and is idempotent when none was recorded.
func TestRemoveLCPPair(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	ch.requests = nil
	if err := b.RemoveLCPPair("loop0"); err != nil {
		t.Fatalf("RemoveLCPPair: %v", err)
	}
	del, ok := ch.requests[0].(*lcp.LcpItfPairAddDel)
	if !ok {
		t.Fatalf("request type: got %T", ch.requests[0])
	}
	if del.IsAdd {
		t.Error("delete: IsAdd got true, want false")
	}
	// Idempotent: removing again issues nothing.
	ch.requests = nil
	if err := b.RemoveLCPPair("loop0"); err != nil {
		t.Fatalf("RemoveLCPPair (second): %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("second RemoveLCPPair: expected no request, got %d", len(ch.requests))
	}
}

// TestDeleteInterfaceRemovesLCPPair verifies deleting a shadowed loopback tears
// down its LCP pair first (the pair references the sw_if_index).
func TestDeleteInterfaceRemovesLCPPair(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	if err := b.SetupLCPPair("loop0", "loop0"); err != nil {
		t.Fatalf("SetupLCPPair: %v", err)
	}
	ch.requests = nil
	if err := b.DeleteInterface("loop0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	var sawLCPDel bool
	for _, r := range ch.requests {
		if d, ok := r.(*lcp.LcpItfPairAddDel); ok && !d.IsAdd {
			sawLCPDel = true
		}
	}
	if !sawLCPDel {
		t.Error("DeleteInterface did not remove the LCP pair")
	}
}
