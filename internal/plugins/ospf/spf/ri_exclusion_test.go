// VALIDATES: spec-ospf-ext-3 AC-17/A-9 -- the RFC 7770 Router Information LSA is
// informational only: it never becomes an SPF vertex (BuildGraph decodes only Router/Network
// LSAs) and never yields an external route (the AS-external computation is gated on the
// precise ASExternal() function-code-5 test, which excludes the AS-scope RI LSA 0xC00C even
// though it is AS-wide).
// PREVENTS: an AS-scope RI LSA sharing the AS-wide store being mis-processed as an
// AS-External route or a topology vertex, corrupting the route table.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// riSource is a minimal SPF Source carrying a fixed set of LSA headers per area plus their
// LSAs, for asserting which LSA types the graph and external computation consider.
type riSource struct {
	headers []packet.LSAHeader
	lsas    map[types.LSAKey]packet.LSA
}

func (s *riSource) Summary(types.AreaID) []packet.LSAHeader { return s.headers }
func (s *riSource) LookupLSA(_ types.AreaID, k types.LSAKey) (packet.LSA, bool) {
	l, ok := s.lsas[k]
	return l, ok
}

func riLSAHeader(t *testing.T, adv string) packet.LSAHeader {
	t.Helper()
	return packet.LSAHeader{
		Type:              types.LSType(ospfv3types.LSTypeRouterInformationAS), // 0xC00C
		LinkStateID:       testLSID(t, "0.0.0.0"),                              // Instance 0
		AdvertisingRouter: testRID(t, adv),
		Sequence:          types.InitialSequenceNumber,
	}
}

func TestRINotInSPFGraph(t *testing.T) {
	area := testArea()
	real := routerLSA(t, "1.1.1.1", stubLink(t, "192.0.2.0", 5))
	riHdr := riLSAHeader(t, "9.9.9.9") // an RI LSA from a router with no Router-LSA

	// The RI AS-scope LS Type must classify AS-wide but NOT AS-External (the ext-3 split).
	if !riHdr.Type.ASWide() {
		t.Fatalf("RI AS type %#04x not AS-wide", uint16(riHdr.Type))
	}
	if riHdr.Type.ASExternal() {
		t.Fatalf("RI AS type %#04x wrongly classified AS-External (would feed SPF)", uint16(riHdr.Type))
	}

	src := &riSource{
		headers: []packet.LSAHeader{real.Header, riHdr},
		lsas:    map[types.LSAKey]packet.LSA{real.Header.Key(): real},
	}

	g := BuildGraph(src, area)
	if _, ok := g.Routers[testRID(t, "9.9.9.9")]; ok {
		t.Fatalf("RI advertising router became a router vertex")
	}
	if len(g.Routers) != 1 || len(g.Networks) != 0 {
		t.Fatalf("graph = %d routers / %d networks, want exactly the one real router", len(g.Routers), len(g.Networks))
	}

	// The AS-external computation must not produce a route from the RI LSA: the ASExternal()
	// guard skips it before LookupLSA is ever consulted, so no reader runs on the RI body.
	border := []BorderRouterEntry{{Kind: BorderRouterASBR, RouterID: testRID(t, "9.9.9.9"), Metric: 10, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}}}
	routes := ComputeExternal(ExternalInput{Source: src, Root: testRID(t, "1.1.1.1"), BorderRouters: border, MaxPaths: 8})
	if len(routes) != 0 {
		t.Fatalf("RI LSA produced %d external route(s), want 0 (informational only)", len(routes))
	}
}
