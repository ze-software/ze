// VALIDATES: spec-ospf-ext-2 TE origination -- OnOriginate builds a Router-Address LSA
// (Instance 0) plus one Link LSA per TE link from config + the live interface snapshot,
// with the Link ID / local / remote filled from the snapshot; refuses a Link TLV without a
// usable Link ID; assigns distinct Instances per link; defaults the TE metric to the OSPF
// cost while keeping an explicit TE metric independent of it; re-originates idempotently;
// and honors the RFC 5392 Type 10 vs Type 11 inter-AS scope policy.
// PREVENTS: origination that never reaches the carrier, malformed Link TLVs, colliding
// Instances, a TE metric aliased to the cost, or a wrong inter-AS flooding scope.
package ospf

import (
	"bytes"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// teEngineWithTopology builds an engine from cfg and injects a canned live topology.
func teEngineWithTopology(t *testing.T, cfgJSON string, topo []ospflsdb.InterfaceInfo) *engine {
	t.Helper()
	eng, _ := newRedistEngine(t, cfgJSON)
	eng.teOrig.setTopology(func() []ospflsdb.InterfaceInfo { return topo })
	return eng
}

func decodeOrigTELSA(t *testing.T, o opaqueOrigination) packet.TELSA {
	t.Helper()
	lsa, err := packet.DecodeTELSA(o.Body)
	if err != nil {
		t.Fatalf("decode originated body (id %d): %v", o.OpaqueID, err)
	}
	return lsa
}

const teOrigCfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
  "areas":{"area":{"0":{"area-id":"0"}}},
  "interfaces":{"interface":{
    "eth0":{"name":"eth0","area":"0","network-type":"point-to-point","cost":"7",
      "traffic-engineering":{"enable":true,"max-bandwidth":"1250000000","admin-group":"3"}}}}}}`

func p2pTopo(name string, local [4]byte, nbr types.RouterID, nbrAddr string) ospflsdb.InterfaceInfo {
	return ospflsdb.InterfaceInfo{
		Name: name, AreaID: types.BackboneArea, NetworkType: networkPointToPoint,
		State: "point-to-point", Address: local, RouterID: types.RouterID{1, 1, 1, 1},
		Neighbors: []ospflsdb.NeighborInfo{{RouterID: nbr, Address: naddrForTest(nbrAddr), State: ospflsdb.NeighborStateFull}},
	}
}

func TestTEOriginateLinkTLVFromSnapshot(t *testing.T) {
	eng := teEngineWithTopology(t, teOrigCfg,
		[]ospflsdb.InterfaceInfo{p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")})
	out := eng.teOriginateType1(types.RouterID{1, 1, 1, 1})

	var ra, link *opaqueOrigination
	for i := range out {
		lsa := decodeOrigTELSA(t, out[i])
		switch {
		case lsa.IsRouterAddress:
			ra = &out[i]
		case lsa.IsLink:
			link = &out[i]
		}
	}
	if ra == nil || ra.OpaqueID != 0 {
		t.Fatalf("router-address must be originated at Instance 0: %+v", ra)
	}
	if link == nil {
		t.Fatalf("no Link LSA originated")
	}
	l := decodeOrigTELSA(t, *link).Link
	if !l.HasLinkType || l.LinkType != packet.TELinkTypePointToPoint {
		t.Fatalf("link type = %v/%d", l.HasLinkType, l.LinkType)
	}
	if !l.HasLinkID || l.LinkID != [4]byte{2, 2, 2, 2} {
		t.Fatalf("link id = %v, want neighbor router id 2.2.2.2", l.LinkID)
	}
	if len(l.LocalIPs) != 1 || l.LocalIPs[0] != [4]byte{10, 0, 0, 1} {
		t.Fatalf("local ip = %v, want 10.0.0.1", l.LocalIPs)
	}
	if len(l.RemoteIPs) != 1 || l.RemoteIPs[0] != [4]byte{10, 0, 0, 2} {
		t.Fatalf("remote ip = %v, want 10.0.0.2", l.RemoteIPs)
	}
	if !l.HasMaxBandwidth || l.MaxBandwidth != float64(float32(1.25e9)) {
		t.Fatalf("max bandwidth = %v/%g", l.HasMaxBandwidth, l.MaxBandwidth)
	}
	if !l.HasAdminGroup || l.AdminGroup != 3 {
		t.Fatalf("admin group = %v/%d", l.HasAdminGroup, l.AdminGroup)
	}
}

func TestTEOriginateRefusesIncompleteLink(t *testing.T) {
	// TE enabled but the p2p interface has no Full neighbor -> no usable Link ID -> no Link
	// LSA (RFC 3630 sec 2.4.2 mandatory Link ID). The router-address may still be originated.
	eng := teEngineWithTopology(t, teOrigCfg, []ospflsdb.InterfaceInfo{{
		Name: "eth0", AreaID: types.BackboneArea, NetworkType: networkPointToPoint,
		State: ospflsdb.InterfaceStateDown, Address: [4]byte{10, 0, 0, 1},
	}})
	for _, o := range eng.teOriginateType1(types.RouterID{1, 1, 1, 1}) {
		if lsa := decodeOrigTELSA(t, o); lsa.IsLink {
			t.Fatalf("originated a Link LSA without a usable Link ID")
		}
	}
}

func TestTEMultipleLinksDistinctInstance(t *testing.T) {
	const cfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{
	    "eth0":{"name":"eth0","area":"0","network-type":"point-to-point","traffic-engineering":{"enable":true}},
	    "eth1":{"name":"eth1","area":"0","network-type":"point-to-point","traffic-engineering":{"enable":true}}}}}}`
	eng := teEngineWithTopology(t, cfg, []ospflsdb.InterfaceInfo{
		p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2"),
		p2pTopo("eth1", [4]byte{10, 0, 1, 1}, types.RouterID{3, 3, 3, 3}, "10.0.1.3"),
	})
	out := eng.teOriginateType1(types.RouterID{1, 1, 1, 1})
	seen := map[uint32]bool{}
	links := 0
	for _, o := range out {
		if seen[o.OpaqueID] {
			t.Fatalf("Instance %d reused across LSAs", o.OpaqueID)
		}
		seen[o.OpaqueID] = true
		if decodeOrigTELSA(t, o).IsLink {
			links++
		}
	}
	if links != 2 {
		t.Fatalf("originated %d Link LSAs, want 2", links)
	}
	if !seen[0] {
		t.Fatalf("router-address (Instance 0) not originated")
	}
}

func TestTEMetricIndependentOfCost(t *testing.T) {
	// eth0: no te-metric -> defaults to the OSPF cost (7). eth1: te-metric 500, cost 7 -> the
	// TE metric is independent (RFC 3630 sec 2.5.5).
	const cfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{
	    "eth0":{"name":"eth0","area":"0","network-type":"point-to-point","cost":"7","traffic-engineering":{"enable":true}},
	    "eth1":{"name":"eth1","area":"0","network-type":"point-to-point","cost":"7","traffic-engineering":{"enable":true,"te-metric":"500"}}}}}}`
	eng := teEngineWithTopology(t, cfg, []ospflsdb.InterfaceInfo{
		p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2"),
		p2pTopo("eth1", [4]byte{10, 0, 1, 1}, types.RouterID{3, 3, 3, 3}, "10.0.1.3"),
	})
	metrics := map[[4]byte]uint32{}
	for _, o := range eng.teOriginateType1(types.RouterID{1, 1, 1, 1}) {
		if lsa := decodeOrigTELSA(t, o); lsa.IsLink && lsa.Link.HasTEMetric {
			metrics[lsa.Link.LinkID] = lsa.Link.TEMetric
		}
	}
	if metrics[[4]byte{2, 2, 2, 2}] != 7 {
		t.Fatalf("eth0 TE metric = %d, want default cost 7", metrics[[4]byte{2, 2, 2, 2}])
	}
	if metrics[[4]byte{3, 3, 3, 3}] != 500 {
		t.Fatalf("eth1 TE metric = %d, want explicit 500", metrics[[4]byte{3, 3, 3, 3}])
	}
}

func TestTEOriginationRateLimited(t *testing.T) {
	// Idempotency: the same config + topology produces byte-identical bodies across passes,
	// so the carrier (which enforces MinLSInterval) refloods nothing (AC-12 / R-9).
	eng := teEngineWithTopology(t, teOrigCfg,
		[]ospflsdb.InterfaceInfo{p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")})
	first := eng.teOriginateType1(types.RouterID{1, 1, 1, 1})
	second := eng.teOriginateType1(types.RouterID{1, 1, 1, 1})
	if len(first) != len(second) {
		t.Fatalf("origination not idempotent: %d then %d LSAs", len(first), len(second))
	}
	byID := map[uint32][]byte{}
	for _, o := range first {
		byID[o.OpaqueID] = o.Body
	}
	for _, o := range second {
		if b, ok := byID[o.OpaqueID]; !ok || !bytes.Equal(b, o.Body) {
			t.Fatalf("Instance %d body changed across identical passes", o.OpaqueID)
		}
		if o.Withdraw {
			t.Fatalf("idempotent pass emitted a withdraw for Instance %d", o.OpaqueID)
		}
	}
}

func TestTEWithdrawOnLinkRemoval(t *testing.T) {
	// A link that disappears from the desired set on the next pass is withdrawn (AC-13).
	eng := teEngineWithTopology(t, teOrigCfg,
		[]ospflsdb.InterfaceInfo{p2pTopo("eth0", [4]byte{10, 0, 0, 1}, types.RouterID{2, 2, 2, 2}, "10.0.0.2")})
	first := eng.teOriginateType1(types.RouterID{1, 1, 1, 1})
	var linkInst uint32
	found := false
	for _, o := range first {
		if decodeOrigTELSA(t, o).IsLink {
			linkInst = o.OpaqueID
			found = true
		}
	}
	if !found {
		t.Fatalf("no link originated on first pass")
	}
	// The interface goes down (empty topology): the next pass must withdraw the link instance.
	eng.teOrig.setTopology(func() []ospflsdb.InterfaceInfo { return nil })
	withdrawn := false
	for _, o := range eng.teOriginateType1(types.RouterID{1, 1, 1, 1}) {
		if o.OpaqueID == linkInst && o.Withdraw {
			withdrawn = true
		}
	}
	if !withdrawn {
		t.Fatalf("link instance %d not withdrawn after the interface went down", linkInst)
	}
}

func TestInterAsTEOriginateScopePolicy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope string
		want  OpaqueScope
	}{
		{"area", "area", OpaqueScopeArea},
		{"as", "as", OpaqueScopeAS},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
			  "areas":{"area":{"0":{"area-id":"0"}}},
			  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point",
			    "traffic-engineering":{"enable":true,"inter-as":{"remote-as":"65001","remote-asbr-ipv4":"203.0.113.9","scope":"` + tc.scope + `"}}}}}}}`
			eng, _ := newRedistEngine(t, cfg)
			out := eng.teOriginateType6(types.RouterID{1, 1, 1, 1})
			var link *opaqueOrigination
			for i := range out {
				if !out[i].Withdraw {
					link = &out[i]
				}
			}
			if link == nil {
				t.Fatalf("no inter-AS Link LSA originated")
			}
			if link.Scope != tc.want {
				t.Fatalf("inter-AS scope = %v, want %v", link.Scope, tc.want)
			}
			l := decodeOrigTELSA(t, *link).Link
			// RFC requirement: RFC5392-3.2.1-1 positive -- the OSPFv2 inter-AS origination never
			// sets HasLinkID, so it never emits the prohibited Link ID sub-TLV (type 2) (§3.2.1).
			if l.HasLinkID {
				t.Fatalf("inter-AS Link TLV must omit the Link ID sub-TLV (RFC 5392 sec 3.2.1)")
			}
			// RFC requirement: RFC5392-3.2.1-4 positive -- origination always emits the REQUIRED
			// Remote AS Number sub-TLV (21) for an inter-AS TE link (§3.2.1, §3.3.1).
			// RFC requirement: RFC5392-3.3.2-1 positive -- origination emits the IPv4 Remote ASBR
			// ID sub-TLV (22) when remote-asbr-ipv4 is configured on the inter-AS link (§3.3.2).
			if !l.HasRemoteAS || l.RemoteAS != 65001 || !l.HasRemoteASBRv4 {
				t.Fatalf("inter-AS Link missing remote AS / ASBR: %+v", l)
			}
		})
	}
}

func TestInterASTEOriginatesWithoutNeighbor(t *testing.T) {
	// RFC 5392 sec 4: an inter-AS TE link carries no OSPF adjacency and exchanges no Hello; the
	// ASBR proxies the advertisement purely from config. Drive origination with an EMPTY live
	// topology (no interfaces, no neighbors, no FSM state) and assert the inter-AS Link LSA is
	// still originated from config alone.
	const cfg = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point",
	    "traffic-engineering":{"enable":true,"inter-as":{"remote-as":"65001","remote-asbr-ipv4":"203.0.113.9"}}}}}}}`
	eng := teEngineWithTopology(t, cfg, nil)
	out := eng.teOriginateType6(types.RouterID{1, 1, 1, 1})
	var link *opaqueOrigination
	for i := range out {
		if !out[i].Withdraw {
			link = &out[i]
		}
	}
	// RFC requirement: RFC5392-4-1 positive -- inter-AS TE origination proceeds from config with
	// no neighbor in the topology, so the inter-AS link forms no adjacency and exchanges no Hello
	// (config-only proxy: teOriginateType6 / buildInterASTELink require no neighbor) (§4).
	// RFC requirement: RFC5392-4-2 positive -- the inter-AS advertisement is produced purely from
	// config with no FSM/adjacency: the LSA originates even though no interface is up (§4).
	if link == nil {
		t.Fatalf("inter-AS Link LSA not originated from config alone (no neighbor / no adjacency)")
	}
	l := decodeOrigTELSA(t, *link).Link
	if !l.HasRemoteAS || l.RemoteAS != 65001 {
		t.Fatalf("config-only inter-AS origination missing Remote AS: %+v", l)
	}
}
