// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- transit traffic, fragment gathering
// Related: xfrm_state_scope_linux.go -- GathersForwardFragments, the answer under test

//go:build linux

package dataplane

import (
	"errors"
	"testing"
)

// xfrmCompleteFeatures is every feature bit K1, K2 and K3 advertise together, bit 4
// (strict ICMP final-policy mapping) aside.
const xfrmCompleteFeatures = xfrmZECapStateScope | xfrmZECapICMPQuote | xfrmZECapFragmentContext |
	xfrmZECapFragmentGuard | xfrmZECapICMPReverse | xfrmZECapICMPForwardMap | xfrmZECapICMPForwardGate

// readReceipt makes xfrmScopeCapsRead answer a receipt of Ze's ABI carrying features.
func readReceipt(features xfrmZEFeature) func() (*xfrmZECapabilities, error) {
	return func() (*xfrmZECapabilities, error) {
		return &xfrmZECapabilities{Features: features, MaxSelectors: saStateScopeMaxSelectors, ABI: xfrmZEABI}, nil
	}
}

// VALIDATES: the XFRM backend answers whether its forward check sees whole datagrams
// from K3's capability bit 3 alone: true with the bit, false without it, false for a
// kernel that sends no receipt (which proves no guard), and the read's error rather
// than false when the receipt cannot be read. The first answer is kept, and a failure
// keeps nothing.
// METHOD: GathersForwardFragments on fresh backends over a stubbed receipt read
// (xfrmScopeCapsRead): every bit, every bit but 3, no receipt; then one backend whose
// read fails, then succeeds with bit 3, then fails again.
// PREVENTS: a port-scoped bypass projected onto FWD on a kernel that classifies
// forwarded fragments before reassembly; an unreadable receipt answered as "does not
// gather", which a caller cannot tell from the kernel's answer; and an install and a
// removal of one entry deriving different policy sets because a later read failed.
func TestXFRMGathersForwardFragmentsReadsCapabilityBit3(t *testing.T) {
	saved := xfrmScopeCapsRead
	t.Cleanup(func() { xfrmScopeCapsRead = saved })

	for _, tc := range []struct {
		name string
		read func() (*xfrmZECapabilities, error)
		want bool
	}{
		{"every bit", readReceipt(xfrmCompleteFeatures), true},
		{"bit 3 alone", readReceipt(xfrmZECapFragmentGuard), true},
		{"every bit but 3", readReceipt(xfrmCompleteFeatures &^ xfrmZECapFragmentGuard), false},
		{"no receipt", func() (*xfrmZECapabilities, error) { return nil, errXFRMZECapabilitiesAbsent }, false},
	} {
		xfrmScopeCapsRead = tc.read
		got, err := (&xfrmBackend{}).GathersForwardFragments()
		if err != nil {
			t.Errorf("%s: the answer failed: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: gathers = %v, want %v", tc.name, got, tc.want)
		}
	}

	errRead := errors.New("GETDEFAULT refused by test")
	failing := func() (*xfrmZECapabilities, error) { return nil, errRead }
	b := &xfrmBackend{}
	xfrmScopeCapsRead = failing
	got, err := b.GathersForwardFragments()
	if !errors.Is(err, errRead) {
		t.Fatalf("an unreadable receipt answered %v with error %v, want the read's error", got, err)
	}
	if got {
		t.Fatal("an unreadable receipt answered that the kernel gathers")
	}
	xfrmScopeCapsRead = readReceipt(xfrmCompleteFeatures)
	if got, err := b.GathersForwardFragments(); err != nil || !got {
		t.Fatalf("after a failed read the next read answered %v, %v; the failure must keep nothing", got, err)
	}
	xfrmScopeCapsRead = failing
	if got, err := b.GathersForwardFragments(); err != nil || !got {
		t.Fatalf("a read failing after the kernel answered changed the answer to %v, %v; the first answer must be kept", got, err)
	}
}
