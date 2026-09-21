// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install
// Related: child.go -- installChildSA, the producer
// RFC: rfc/short/rfc4301.md -- ESP support (Section 3.2)
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: RFC4301-3.2-1. Every IKE-keyed Child SA is installed as ESP: both the
// inbound and the outbound security association carry IP protocol 50 and an encryption
// transform with key material, so the SA ze builds is an ESP SA and not an AH one.
// PREVENTS: a Child SA that reaches the kernel under another protocol number, which the
// peer, having negotiated ESP, would drop.
// RFC requirement: RFC4301-3.2-1 positive -- an IKE-keyed Child SA installs an inbound and an outbound ESP (protocol 50) association carrying an encryption key.
func TestRFC4301ChildSAIsInstalledAsESP(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 42, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2 (inbound + outbound)", len(dp.sas))
	}
	seen := map[dataplane.SADir]bool{}
	for i, s := range dp.sas {
		if s.Proto != 50 {
			t.Errorf("SA[%d] protocol = %d, want ESP (50)", i, s.Proto)
		}
		if s.EncAlgo == "" || len(s.EncKey) == 0 {
			t.Errorf("SA[%d] carries no encryption transform (%q, %d key octets)", i, s.EncAlgo, len(s.EncKey))
		}
		seen[s.Dir] = true
	}
	if !seen[dataplane.SADirIn] || !seen[dataplane.SADirOut] {
		t.Fatalf("directions installed = %v, want inbound and outbound", seen)
	}
}
