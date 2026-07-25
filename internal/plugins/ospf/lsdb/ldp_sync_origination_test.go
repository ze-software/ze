// VALIDATES: spec-ospf-ext-11 -- RFC 6138 §4 broadcast transit-link withhold and the
// RFC 5443 §2 P2P LSInfinity cost-out, both driven from the shared InterfaceInfo model
// at the single routerLinks() origination seam.
// PREVENTS: withholding a cut-edge (network partition), corrupting the rest of the
// Router-LSA when withholding the transit link, confusing the P2P cost-out (which uses
// LSInfinity, never withhold) with the broadcast withhold, and (review FIX 2) costing
// out the connected-subnet STUB link along with the p2p link.
package lsdb

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func broadcastLDPSyncTopology(withhold bool) []InterfaceInfo {
	return []InterfaceInfo{{
		Name:                   "eth0",
		AreaID:                 area("0.0.0.0"),
		AreaType:               AreaTypeNormal,
		NetworkType:            NetworkBroadcast,
		State:                  InterfaceStateDR,
		Address:                ip4("10.0.0.1"),
		NetworkMask:            ip4("255.255.255.0"),
		Cost:                   10,
		RouterID:               rid("1.1.1.1"),
		Options:                types.OptionE,
		DR:                     rid("1.1.1.1"),
		BDR:                    rid("2.2.2.2"),
		Neighbors:              []NeighborInfo{{RouterID: rid("2.2.2.2"), Address: naddr4("10.0.0.2"), State: NeighborStateFull}},
		LDPSyncWithholdTransit: withhold,
	}}
}

func routerLinksByType(t *testing.T, in OriginInput, typ uint8) []packet.RouterLink {
	t.Helper()
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	h, ok := db.OriginateRouter(in)
	if !ok {
		t.Fatalf("OriginateRouter returned false")
	}
	lsa, ok := db.LookupLSA(in.AreaID, h.Key())
	if !ok {
		t.Fatalf("originated Router-LSA not installed")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	var out []packet.RouterLink
	for _, l := range body.Links {
		if l.Type == typ {
			out = append(out, l)
		}
	}
	return out
}

func TestLDPSyncBroadcastWithholdsTransitLink(t *testing.T) {
	// AC-6 / A-7: a non-cut-edge broadcast interface withholds ONLY the transit
	// (Link Type 2) link; the stub link for the subnet is still advertised.
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: broadcastLDPSyncTopology(true)}
	if transit := routerLinksByType(t, in, packet.RouterLinkTypeTransit); len(transit) != 0 {
		t.Fatalf("transit links = %+v, want none withheld (RFC 6138 §4)", transit)
	}
	stub := routerLinksByType(t, in, packet.RouterLinkTypeStub)
	if len(stub) != 1 {
		t.Fatalf("stub links = %+v, want the subnet stub retained", stub)
	}
	if stub[0].Metric != 10 {
		t.Fatalf("stub metric = %d, want the configured 10 (withhold does not cost-out)", stub[0].Metric)
	}
}

func TestLDPSyncBroadcastCutEdgeAdvertised(t *testing.T) {
	// AC-7 / R-3: a cut-edge broadcast interface (withhold flag never set by the
	// engine for a cut-edge) advertises the transit link immediately at normal cost.
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: broadcastLDPSyncTopology(false)}
	transit := routerLinksByType(t, in, packet.RouterLinkTypeTransit)
	if len(transit) != 1 {
		t.Fatalf("transit links = %+v, want the transit link advertised immediately (RFC 6138 §4 MUST NOT delay)", transit)
	}
	if transit[0].Metric != 10 {
		t.Fatalf("transit metric = %d, want the configured 10 (cut-edge advertised at normal cost)", transit[0].Metric)
	}
}

func TestLDPSyncP2PMaxMetricNotWithheld(t *testing.T) {
	// A-7 + review FIX 2: a not-synchronized P2P interface has its p2p LINK cost-out to
	// LSInfinity (RFC 5443 §2) and is NOT withheld (withhold is broadcast-only), while
	// its connected-subnet STUB link keeps the configured cost. The engine sets the
	// per-interface LDPSyncMaxMetric flag (NOT a blanket info.Cost = LSInfinity, which
	// would also cost out the stub), so only the p2p/transit link is raised -- mirroring
	// the RFC 6987 router-wide max-metric path.
	const configured = 10
	topo := []InterfaceInfo{{
		Name:             "ptp0",
		AreaID:           area("0.0.0.0"),
		AreaType:         AreaTypeNormal,
		NetworkType:      NetworkPointToPoint,
		State:            "point-to-point",
		Address:          ip4("192.0.2.1"),
		NetworkMask:      ip4("255.255.255.252"),
		Cost:             configured,
		RouterID:         rid("1.1.1.1"),
		Options:          types.OptionE,
		Neighbors:        []NeighborInfo{{RouterID: rid("3.3.3.3"), Address: naddr4("192.0.2.2"), State: NeighborStateFull}},
		LDPSyncMaxMetric: true,
	}}
	in := OriginInput{AreaID: area("0.0.0.0"), RouterID: rid("1.1.1.1"), Options: types.OptionE, Interfaces: topo}
	p2p := routerLinksByType(t, in, packet.RouterLinkTypeP2P)
	if len(p2p) != 1 {
		t.Fatalf("p2p links = %+v, want the link present (P2P is cost-out, never withheld)", p2p)
	}
	if p2p[0].Metric != LSInfinity {
		t.Fatalf("p2p metric = %d, want LSInfinity %d", p2p[0].Metric, uint16(LSInfinity))
	}
	stub := routerLinksByType(t, in, packet.RouterLinkTypeStub)
	if len(stub) != 1 {
		t.Fatalf("stub links = %+v, want the connected-subnet stub retained", stub)
	}
	if stub[0].Metric != configured {
		t.Fatalf("stub metric = %d, want the configured %d (FIX 2: only the p2p link is cost-out, not the stub)", stub[0].Metric, configured)
	}
}
