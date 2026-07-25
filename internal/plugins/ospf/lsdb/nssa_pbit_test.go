// VALIDATES: spec-ospf-14 AC-12 -- RFC 3101 §2.3/§2.4 Type 7 P-bit policy enforced at the
// OriginateNSSA boundary: the P-bit requires a non-zero forwarding address and must be clear
// when the same network is also originated as a Type 5 AS-External LSA.
// PREVENTS: a caller injecting a Type 7 with P=1 and a zero forwarding address, or with P=1
// while this router already advertises a Type 5 for the same network (double translation).
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFNSSAPBitBoundaryPolicy(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock) // self router = 1.1.1.1
	a := area("0.0.0.1")
	self := rid("1.1.1.1")
	mask := ip4("255.255.255.0")
	fa := ip4("10.0.0.9")

	// P=1 with a zero forwarding address -> P cleared (RFC 3101 §2.3).
	// RFC requirement: RFC3101-2.4-2 negative -- a P=1 request with a zero forwarding address
	// originates a Type-7 with the P-bit forced clear (a P=1 Type-7 needs a non-zero FA).
	h, _ := db.OriginateNSSA(a, self, ip4("203.0.113.0"), mask, true, 20, [4]byte{}, 0, true)
	if h.Options.Has(types.OptionNP) {
		t.Fatalf("AC-12: P-bit set despite a zero forwarding address")
	}

	// P=1 with a non-zero forwarding address and no Type 5 -> P set.
	// RFC requirement: RFC3101-2.4-2 positive -- a P=1 request with a non-zero forwarding
	// address keeps the P-bit set.
	// RFC requirement: RFC3101-2.4-3 positive -- with no self Type-5 for the network, a P=1
	// Type-7 keeps the P-bit set.
	h, _ = db.OriginateNSSA(a, self, ip4("203.0.113.16"), mask, true, 20, fa, 0, true)
	if !h.Options.Has(types.OptionNP) {
		t.Fatalf("AC-12: P-bit clear despite a non-zero forwarding address and no Type 5")
	}

	// P=1 with a non-zero forwarding address but a self Type 5 for the same network -> P clear.
	// RFC requirement: RFC3101-2.4-3 negative -- when this router also originates a Type-5 for
	// the same network, the Type-7's P-bit is forced clear (no double translation).
	net := ip4("203.0.113.32")
	if _, ok, err := db.OriginateExternal(self, net, mask, 0, true, 20, [4]byte{}, 0); err != nil || !ok {
		t.Fatalf("setup: OriginateExternal failed: ok=%v err=%v", ok, err)
	}
	h, _ = db.OriginateNSSA(a, self, net, mask, true, 20, fa, 0, true)
	if h.Options.Has(types.OptionNP) {
		t.Fatalf("AC-12: P-bit set despite a self Type 5 for the same network")
	}
}
