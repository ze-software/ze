package spf

import (
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func testArea() types.AreaID { return types.BackboneArea }

func testRID(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatalf("ParseRouterID(%q): %v", s, err)
	}
	return id
}

func testLSID(t *testing.T, s string) types.LinkStateID {
	t.Helper()
	id, err := types.ParseLinkStateID(s)
	if err != nil {
		t.Fatalf("ParseLinkStateID(%q): %v", s, err)
	}
	return id
}

func testIP(t *testing.T, s string) [4]byte {
	t.Helper()
	id := testLSID(t, s)
	return [4]byte(id)
}

func metric(t *testing.T, v uint16) types.Metric {
	t.Helper()
	m, err := types.NewMetric(uint32(v))
	if err != nil {
		t.Fatalf("NewMetric(%d): %v", v, err)
	}
	return m
}

func p2pLink(t *testing.T, router, local string, cost uint16) packet.RouterLink {
	t.Helper()
	return packet.RouterLink{LinkID: testLSID(t, router), LinkData: testIP(t, local), Type: packet.RouterLinkTypeP2P, Metric: metric(t, cost)}
}

func transitLink(t *testing.T, local string) packet.RouterLink {
	t.Helper()
	return packet.RouterLink{LinkID: testLSID(t, "10.0.0.254"), LinkData: testIP(t, local), Type: packet.RouterLinkTypeTransit, Metric: metric(t, 1)}
}

// transitLinkDR builds a transit (broadcast) link whose Link ID is an explicit DR
// interface address (the Network-LSA LS ID), for topologies with more than one LAN.
func transitLinkDR(t *testing.T, dr, local string, cost uint16) packet.RouterLink {
	t.Helper()
	return packet.RouterLink{LinkID: testLSID(t, dr), LinkData: testIP(t, local), Type: packet.RouterLinkTypeTransit, Metric: metric(t, cost)}
}

func stubLink(t *testing.T, network string, cost uint16) packet.RouterLink {
	t.Helper()
	return packet.RouterLink{LinkID: testLSID(t, network), LinkData: testIP(t, "255.255.255.0"), Type: packet.RouterLinkTypeStub, Metric: metric(t, cost)}
}

func routerLSA(t *testing.T, router string, links ...packet.RouterLink) packet.LSA {
	t.Helper()
	rid := testRID(t, router)
	return packet.LSA{
		Header: packet.LSAHeader{
			Options:           types.OptionE,
			Type:              types.LSTypeRouter,
			LinkStateID:       linkStateIDFromRouterID(rid),
			AdvertisingRouter: rid,
			Sequence:          types.InitialSequenceNumber,
		},
		Router: &packet.RouterLSA{Links: links},
	}
}

func networkLSA(t *testing.T, drAddr, advRouter, mask string, attached ...string) packet.LSA {
	t.Helper()
	routers := make([]types.RouterID, 0, len(attached))
	for _, r := range attached {
		routers = append(routers, testRID(t, r))
	}
	return packet.LSA{
		Header: packet.LSAHeader{
			Options:           types.OptionE,
			Type:              types.LSTypeNetwork,
			LinkStateID:       testLSID(t, drAddr),
			AdvertisingRouter: testRID(t, advRouter),
			Sequence:          types.InitialSequenceNumber,
		},
		Network: &packet.NetworkLSA{NetworkMask: testIP(t, mask), AttachedRouters: routers},
	}
}

func testSource(t *testing.T, area types.AreaID, lsas ...packet.LSA) *ospflsdb.LSDB {
	t.Helper()
	db := ospflsdb.New(nil)
	for _, lsa := range lsas {
		if !db.Install(area, lsa) {
			t.Fatalf("Install(%s, %s) rejected", area, lsa.Header.Type)
		}
	}
	return db
}

func baseP2PSource(t *testing.T, area types.AreaID) *ospflsdb.LSDB {
	t.Helper()
	return testSource(t, area,
		routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
		routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), stubLink(t, "192.0.2.0", 5)),
	)
}
