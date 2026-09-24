// Design: docs/architecture/ospf/ospf-ext-7-virtual-links.md -- transit changes on a live virtual adjacency.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// virtualRouteEngine establishes a virtual adjacency through the router's Hello/DD dispatcher.
func virtualRouteEngine(t *testing.T) (*engine, *fakeBackend, ospfspf.VirtualNeighborResult) {
	t.Helper()
	const config = `{"ospf":{"router-id":"10.0.0.1","opaque":true,
		"areas":{"area":{"0.0.0.0":{},"0.0.0.1":{"virtual-link":{"10.0.0.2":{}}}}},
		"interfaces":{"interface":{"eth0":{"area":"0.0.0.1","network-type":"point-to-point"},
		"eth1":{"area":"0.0.0.0","network-type":"point-to-point"}}}}}`
	eng, backend := vlEngine(t, config)
	t.Cleanup(eng.shutdown)
	eng.spf.Stop() // The test supplies transit SPF results; no asynchronous SPF may replace them.
	eng.lsdb.SetTimers(ospflsdb.TimerConfig{MinLSInterval: time.Nanosecond})
	result := ospfspf.VirtualNeighborResult{
		TransitArea: types.AreaID{0, 0, 0, 1}, Neighbor: ridOf("10.0.0.2"), Reachable: true, Cost: 10,
		Address:  netip.MustParseAddr("192.0.2.2"),
		NextHops: []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.254"), Interface: "eth0"}},
	}
	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{result})
	return eng, backend, result
}

func establishVirtualRoute(t *testing.T, eng *engine, backend *fakeBackend, result ospfspf.VirtualNeighborResult) string {
	t.Helper()
	index := vlIfindex(t, backend, "eth0")
	dispatchVirtualHello(t, eng, index, result.Neighbor, types.BackboneArea, result.Address, eng.cfg.RouterID)
	options := types.OptionE | types.OptionO
	dispatchDBDesc(t, eng, index, result.Neighbor, types.BackboneArea, packet.DBDesc{Options: options, Flags: packet.DDFlagInit | packet.DDFlagMore | packet.DDFlagMaster, DDSequence: 7})
	dispatchDBDesc(t, eng, index, result.Neighbor, types.BackboneArea, packet.DBDesc{Options: options, Flags: packet.DDFlagMaster, DDSequence: 8})
	name := virtualLinkName(virtualLinkKey{transit: result.TransitArea, neighbor: result.Neighbor})
	rows := eng.neighbors.Snapshot()
	for index := range rows {
		row := &rows[index]
		if row.Interface == name && row.State == ospflsdb.NeighborStateFull {
			// Drain initial Full-state origination without waiting for the maintenance tick.
			eng.originateSelfLSAs()
			return name
		}
	}
	t.Fatalf("virtual adjacency is not Full: %+v", eng.neighbors.Snapshot())
	return ""
}

func virtualRouterMetric(t *testing.T, eng *engine, peer types.RouterID) (uint16, bool) {
	t.Helper()
	router := vlDecodeRouter(t, eng.lsdb, types.BackboneArea, eng.cfg.RouterID)
	for _, link := range router.Links {
		if link.Type == packet.RouterLinkTypeVirtual && link.LinkID == types.LinkStateID(peer) {
			return uint16(link.Metric), true
		}
	}
	return 0, false
}

// RFC requirement: RFC2328-15-2 positive -- changing the transit path of a Full virtual
// adjacency updates its runtime cost and next hop and immediately originates that cost
// in the backbone Router-LSA without recreating the adjacency.
// RFC requirement: RFC4577-4.1.4-2 positive -- an ordinary CE can establish its
// backbone adjacency over a virtual link; real Hello/DD ingress reaches Full and
// the backbone Router-LSA advertises the virtual neighbor.
func TestRFC2328VirtualCostChangeReoriginatesBackbone(t *testing.T) {
	eng, backend, result := virtualRouteEngine(t)
	name := establishVirtualRoute(t, eng, backend, result)
	if metric, present := virtualRouterMetric(t, eng, result.Neighbor); !present || metric != 10 {
		t.Fatalf("initial virtual metric = %d, present=%v", metric, present)
	}
	eng.mu.Lock()
	original := eng.interfaces[name]
	eng.mu.Unlock()
	result.Cost = 77
	result.NextHops = []ospfspf.NextHop{{Addr: netip.MustParseAddr("192.0.2.253"), Interface: "eth0"}}
	eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{result})
	if metric, present := virtualRouterMetric(t, eng, result.Neighbor); !present || metric != 77 {
		t.Fatalf("updated virtual metric = %d, present=%v, want 77", metric, present)
	}
	eng.mu.Lock()
	runtime := eng.virtualLinks[virtualLinkKey{transit: result.TransitArea, neighbor: result.Neighbor}]
	unchanged := runtime.iface == original
	nextHop, destination := runtime.transitNextHop, runtime.neighborAddr
	eng.mu.Unlock()
	if !unchanged || nextHop != result.NextHops[0] || destination != result.Address {
		t.Fatalf("adjacency preserved=%v, next hop=%+v, endpoint=%s", unchanged, nextHop, destination)
	}
}

// RFC requirement: RFC2328-15-2 negative -- an unreachable or unencodable transit path
// removes the virtual link from the backbone Router-LSA instead of retaining its old cost.
func TestRFC2328VirtualUnusablePathWithdrawsBackboneLink(t *testing.T) {
	for _, unreachable := range []bool{false, true} {
		t.Run(map[bool]string{false: "cost-overflow", true: "unreachable"}[unreachable], func(t *testing.T) {
			eng, backend, result := virtualRouteEngine(t)
			establishVirtualRoute(t, eng, backend, result)
			if unreachable {
				result.Reachable = false
			} else {
				result.Cost = 65536
			}
			eng.onVirtualLinksResolved([]ospfspf.VirtualNeighborResult{result})
			if metric, present := virtualRouterMetric(t, eng, result.Neighbor); present {
				t.Fatalf("unusable virtual link still advertised with metric %d", metric)
			}
		})
	}
}
