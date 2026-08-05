// VALIDATES: that the SAD and SPD read paths report what a REAL kernel holds --
// an SA and a policy this test installed appear in the dump with their own
// values, and disappear from it when removed.
// PREVENTS: the vacuity trap the older TestXFRMListSAs falls into. That test
// passes on an empty kernel with the whole read body deleted, because it asserts
// only that whatever came back is well formed. Every assertion below names an
// object this test installed, and asserts a TRANSITION (absent, present, absent)
// rather than a state (ai/rules/interop-and-goal-validation.md).
//
// Design: plan/spec-ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: xfrm_readback_linux_test.go -- the same mapping driven from fixtures

//go:build integration && linux

package dataplane

import (
	"net"
	"testing"
)

// readbackSPI is chosen high and distinctive so a stray SA on the test host
// cannot be mistaken for the one under test.
const (
	readbackSPI   uint32 = 0x5ead0001
	readbackReqID uint32 = 0x5ead
)

// findSA returns the dumped SA with this SPI, or nil.
func findSA(t *testing.T, b *xfrmBackend, spi uint32) *SAInfo {
	t.Helper()
	sas, err := b.ListSAs(0)
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("ListSAs: %v", err)
	}
	for i := range sas {
		if sas[i].SPI == spi {
			return &sas[i]
		}
	}
	return nil
}

// TestXFRMReadbackShowsInstalledSA is the transition assertion for the SAD.
func TestXFRMReadbackShowsInstalledSA(t *testing.T) {
	b := &xfrmBackend{}
	src := net.ParseIP("10.88.0.1")
	dst := net.ParseIP("10.88.0.2")

	if sa := findSA(t, b, readbackSPI); sa != nil {
		t.Fatalf("SPI %#x is already installed before the test ran: %+v", readbackSPI, sa)
	}

	err := b.InstallSA(SAParams{
		SPI: readbackSPI, Src: src, Dst: dst, Proto: ProtoESP, Mode: ModeTunnel,
		ReqID: readbackReqID, ReplayWin: 64,
		EncAlgo: "aes256", EncKey: make([]byte, 32),
		AuthAlgo: "sha256", AuthKey: make([]byte, 32),
	})
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallSA: %v", err)
	}
	t.Cleanup(func() { _ = b.RemoveSA(readbackSPI, dst, ProtoESP) })

	sa := findSA(t, b, readbackSPI)
	if sa == nil {
		t.Fatalf("SPI %#x was installed and the SAD dump does not report it", readbackSPI)
	}

	// Every field below was set by THIS test, so a dump that dropped or
	// mistranslated one is caught here rather than by inspection.
	if !sa.Src.Equal(src) || !sa.Dst.Equal(dst) {
		t.Errorf("addresses = %v -> %v, want %v -> %v", sa.Src, sa.Dst, src, dst)
	}
	if sa.Mode != ModeTunnel {
		t.Errorf("Mode = %d, want ModeTunnel (%d): the kernel numbers tunnel one lower than ze does",
			sa.Mode, ModeTunnel)
	}
	if sa.ReqID != readbackReqID {
		t.Errorf("ReqID = %d, want %d", sa.ReqID, readbackReqID)
	}
	if sa.ReplayWindow != 64 {
		t.Errorf("ReplayWindow = %d, want 64", sa.ReplayWindow)
	}
	if sa.Proto != ProtoESP {
		t.Errorf("Proto = %d, want ESP (%d)", sa.Proto, ProtoESP)
	}
	// A-1: the dump populates the algorithm arms and the counters without a
	// per-SA XFRM_MSG_GETSA round trip.
	if sa.Encryption == "" {
		t.Error("Encryption is empty: the dump did not carry the cipher name")
	}
	if sa.EncryptionKeyBits != 256 {
		t.Errorf("EncryptionKeyBits = %d, want 256", sa.EncryptionKeyBits)
	}
	if sa.Integrity == "" {
		t.Error("Integrity is empty: the dump did not carry the integrity name")
	}
	if sa.AddedAt.IsZero() {
		t.Error("AddedAt is the zero time: the kernel records an add time for every SA it accepts")
	}
	// A fresh SA has carried nothing, so UsedAt must be the zero time rather
	// than 1970.
	if !sa.UsedAt.IsZero() {
		t.Errorf("UsedAt = %v, want the zero time for an SA that has carried no packet", sa.UsedAt)
	}

	// The removal half. Without it the test would prove only that a dump lists
	// something, not that it TRACKS the kernel.
	if err := b.RemoveSA(readbackSPI, dst, ProtoESP); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}
	if sa := findSA(t, b, readbackSPI); sa != nil {
		t.Errorf("SPI %#x was removed and the SAD dump still reports it: %+v", readbackSPI, sa)
	}
}

// TestXFRMReadbackIfIDFilter proves the filter argument against a real dump.
func TestXFRMReadbackIfIDFilter(t *testing.T) {
	b := &xfrmBackend{}
	src := net.ParseIP("10.88.1.1")
	dst := net.ParseIP("10.88.1.2")
	const spi uint32 = 0x5ead0002
	const ifID uint32 = 0x5e01

	err := b.InstallSA(SAParams{
		SPI: spi, Src: src, Dst: dst, Proto: ProtoESP, Mode: ModeTunnel,
		ReqID: readbackReqID, IfID: ifID,
		EncAlgo: "aes256", EncKey: make([]byte, 32),
		AuthAlgo: "sha256", AuthKey: make([]byte, 32),
	})
	if err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallSA: %v", err)
	}
	t.Cleanup(func() { _ = b.RemoveSA(spi, dst, ProtoESP) })

	matching, err := b.ListSAs(ifID)
	if err != nil {
		t.Fatalf("ListSAs(%d): %v", ifID, err)
	}
	var seen bool
	for i := range matching {
		if matching[i].SPI == spi {
			seen = true
		}
		if matching[i].IfID != ifID {
			t.Errorf("ListSAs(%d) returned an SA with if_id %d", ifID, matching[i].IfID)
		}
	}
	if !seen {
		t.Fatalf("ListSAs(%d) did not return the SA installed with that if_id", ifID)
	}

	// A DIFFERENT if_id must not return it. Without this the filter could be a
	// no-op and the assertion above would still hold.
	other, err := b.ListSAs(ifID + 1)
	if err != nil {
		t.Fatalf("ListSAs(%d): %v", ifID+1, err)
	}
	for i := range other {
		if other[i].SPI == spi {
			t.Errorf("ListSAs(%d) returned the SA installed at if_id %d, so the filter does nothing", ifID+1, ifID)
		}
	}
}

// TestXFRMReadbackPolicyNoIfID is A-5's evidence: the SPD dump is NOT limited to
// xfrm-interface peers, so it reports the site-to-site policies IKE installs at
// if_id 0. If it were limited, the dump would miss most policies in the field.
func TestXFRMReadbackPolicyNoIfID(t *testing.T) {
	b := &xfrmBackend{}
	selectorSrc := mustCIDR(t, "10.88.2.0/24")
	selectorDst := mustCIDR(t, "10.88.3.0/24")

	params := SPParams{
		Src: selectorSrc, Dst: selectorDst, Dir: SADirOut,
		Proto: ProtoESP, Mode: ModeTunnel,
		ReqID:     readbackReqID,
		TunnelSrc: net.ParseIP("10.88.2.1"),
		TunnelDst: net.ParseIP("10.88.3.1"),
		Owner:     "readback-peer",
	}
	if err := b.InstallPolicy(params); err != nil {
		skipUnprivileged(t, err)
		t.Fatalf("InstallPolicy: %v", err)
	}
	t.Cleanup(func() { _ = b.RemovePolicyParams(params) })

	policies, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	var found *PolicyInfo
	for i := range policies {
		p := &policies[i]
		if p.Src != nil && p.Dst != nil &&
			p.Src.String() == selectorSrc.String() && p.Dst.String() == selectorDst.String() &&
			p.Dir == SADirOut {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("a policy installed at if_id 0 is absent from the SPD dump, so the dump misses site-to-site policies")
	}
	if found.IfID != 0 {
		t.Errorf("IfID = %d, want 0", found.IfID)
	}
	if found.Mode != ModeTunnel {
		t.Errorf("Mode = %d, want ModeTunnel", found.Mode)
	}
	if found.ReqID != readbackReqID {
		t.Errorf("ReqID = %d, want %d", found.ReqID, readbackReqID)
	}
	if !found.TunnelSrc.Equal(params.TunnelSrc) || !found.TunnelDst.Equal(params.TunnelDst) {
		t.Errorf("template endpoints = %v -> %v, want %v -> %v",
			found.TunnelSrc, found.TunnelDst, params.TunnelSrc, params.TunnelDst)
	}
	// The owner join across a REAL kernel round trip. This is A-7 proven against
	// the kernel's own normalization rather than against a fixture.
	if !found.OwnerKnown || found.Owner != "readback-peer" {
		t.Errorf("owner = %q (known=%v), want readback-peer: the kernel row did not resolve back to the policy ze installed",
			found.Owner, found.OwnerKnown)
	}

	if err := b.RemovePolicyParams(params); err != nil {
		t.Fatalf("RemovePolicyParams: %v", err)
	}
	after, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("ListPolicies after remove: %v", err)
	}
	for i := range after {
		p := &after[i]
		if p.Src != nil && p.Src.String() == selectorSrc.String() && p.Dir == SADirOut {
			t.Errorf("the policy was removed and the SPD dump still reports it: %+v", p)
		}
	}
}
