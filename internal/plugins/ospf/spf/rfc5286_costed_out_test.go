// VALIDATES: RFC 5286 Section 3.5 costed-out link exclusion at the value a real OSPF
// Router-LSA carries: a link metric is 16 bits, so a costed-out link (RFC 6987
// MaxLinkMetric, the RFC 3137 "LSInfinity" of a Router-LSA link) advertises 0xffff.
// PREVENTS: comparing a 16-bit link metric against the 24-bit summary LSInfinity
// (0xffffff), a test no Router-LSA link can ever meet, so a costed-out neighbor is
// used as a loop-free alternate.
package spf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// TestRFC5286CostedOutLinkMetric drives selectLFA with the forward and reverse costs a
// Router-LSA can actually carry. Method: one loop-free alternate, then the same case with
// the forward cost, the reverse cost, and every reverse link set to 0xffff.
func TestRFC5286CostedOutLinkMetric(t *testing.T) {
	loopFree := selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 5, altN: 10, destD: 15},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10},
		dE:  map[string]uint64{nbrE: 0, destD: 10},
		fwd: 0xfffe, rev: 0xfffe,
	}

	// RFC requirement: RFC5286-x-2 positive -- the largest link metric below the costed-out
	// value (0xfffe) is a finite cost, so the loop-free alternate over it stays usable.
	if _, ok := runSelect(t, loopFree); !ok {
		t.Fatalf("alternate over a 0xfffe link was rejected; only 0xffff is costed out")
	}

	// RFC requirement: RFC5286-x-2 negative -- a link whose forward cost is 0xffff, or
	// whose reverse cost is 0xffff, is costed out and MUST NOT carry an alternate.
	fwdOut := loopFree
	fwdOut.fwd = 0xffff
	if _, ok := runSelect(t, fwdOut); ok {
		t.Fatalf("alternate over a link with forward cost 0xffff was used; RFC 5286 Section 3.5 forbids it")
	}
	revOut := loopFree
	revOut.rev = 0xffff
	if _, ok := runSelect(t, revOut); ok {
		t.Fatalf("alternate over a link with reverse cost 0xffff was used; RFC 5286 Section 3.5 forbids it")
	}
}

// TestRFC5286AllReverseLinksCostedOut builds N's Router-LSA with parallel links back to
// S. Method: reverseP2PCost reduces them to their minimum, and selectLFA judges that
// minimum; when every link back is 0xffff the neighbor is excluded.
func TestRFC5286AllReverseLinksCostedOut(t *testing.T) {
	root := testRID(t, srcS)
	nb := testRID(t, altN)
	loopFree := selectCase{
		dS:  map[string]uint64{srcS: 0, nbrE: 5, altN: 10, destD: 15},
		dN:  map[string]uint64{altN: 0, srcS: 10, nbrE: 15, destD: 10},
		dE:  map[string]uint64{nbrE: 0, destD: 10},
		fwd: 10,
	}

	// RFC requirement: RFC5286-x-3 positive -- one of N's links back to S is finite, so the
	// reverse cost is that link's metric and N stays usable.
	gOne := NewGraph(testArea())
	gOne.Routers[nb] = &RouterVertex{ID: nb, Links: []packet.RouterLink{
		p2pLink(t, srcS, "10.0.13.2", 0xffff),
		p2pLink(t, srcS, "10.0.13.6", 20),
	}}
	usable := loopFree
	usable.rev = reverseP2PCost(gOne, nb, root)
	if _, ok := runSelect(t, usable); !ok {
		t.Fatalf("neighbor with one finite link back to S (reverse cost %d) was rejected", usable.rev)
	}

	// RFC requirement: RFC5286-x-3 negative -- every link from N back to S is costed out at
	// 0xffff, so S MUST NOT use N as an alternate.
	gAll := NewGraph(testArea())
	gAll.Routers[nb] = &RouterVertex{ID: nb, Links: []packet.RouterLink{
		p2pLink(t, srcS, "10.0.13.2", 0xffff),
		p2pLink(t, srcS, "10.0.13.6", 0xffff),
	}}
	excluded := loopFree
	excluded.rev = reverseP2PCost(gAll, nb, root)
	if _, ok := runSelect(t, excluded); ok {
		t.Fatalf("neighbor whose every link back to S costs 0xffff was used; RFC 5286 Section 3.5 forbids it")
	}
}
