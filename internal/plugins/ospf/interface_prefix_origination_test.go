// Design: docs/architecture/ospf/ospf-4-component-config.md -- interface address inventory.
// Related: lsdb/origination.go -- Router-LSA stub links.
package ospf

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const prefixOriginationBackendName = "ospf-prefix-origination-test"

// prefixOriginationBackend substitutes only the live interface inventory. The
// engine still parses configuration, enrolls interfaces and originates its LSAs.
type prefixOriginationBackend struct {
	rfc7474AddressBackend
	mu          sync.Mutex
	failName    string
	failAll     bool
	failNthName string
	failNthRead uint64
	namedReads  uint64
	faultRead   bool
	// overrideAddresses is nil unless a boundary test supplies its own inventory.
	overrideAddresses []iface.AddrInfo
	expanded          bool
	blockName         string
	readStarted       chan struct{}
	releaseRead       <-chan struct{}
}

func (b *prefixOriginationBackend) GetInterface(name string) (*iface.InterfaceInfo, error) {
	b.mu.Lock()
	if name == b.blockName {
		b.blockName = ""
		started, release := b.readStarted, b.releaseRead
		b.mu.Unlock()
		close(started)
		<-release
		b.mu.Lock()
	}
	defer b.mu.Unlock()
	if b.failAll {
		return nil, fmt.Errorf("test interface %s address inventory unavailable", name)
	}
	if name == b.failName {
		return nil, fmt.Errorf("test interface %s address inventory unavailable", name)
	}
	if b.failNthRead > 0 {
		if name == b.failNthName {
			b.namedReads++
			if b.namedReads == b.failNthRead {
				b.faultRead = true
				return nil, fmt.Errorf("test interface %s address inventory unavailable on read %d", name, b.namedReads)
			}
		}
	}
	info := iface.InterfaceInfo{Name: name, OsName: name, Index: 1, State: iface.LinkStateUp, MTU: 1500, AddressesRead: true}
	switch name {
	case "lo":
		info.Type = "loopback"
		info.Addresses = []iface.AddrInfo{
			{Address: "127.0.0.1", PrefixLength: 8, Family: "ipv4"},
			{Address: "10.0.0.1", PrefixLength: 32, Family: "ipv4"},
			{Address: "10.0.0.9", PrefixLength: 24, Family: "ipv4"},
		}
		if b.expanded {
			info.Addresses = append(info.Addresses, iface.AddrInfo{Address: "10.0.0.99", PrefixLength: 32, Family: "ipv4"})
		}
	case "lan0":
		info.Type = "veth"
		info.Addresses = []iface.AddrInfo{
			{Address: "192.0.2.1", PrefixLength: 24, Family: "ipv4"},
			{Address: "192.0.2.1", PrefixLength: 25, Family: "ipv4"},
			{Address: "198.51.100.1", PrefixLength: 25, Family: "ipv4"},
			{Address: "198.51.100.2", PrefixLength: 25, Family: "ipv4"},
			{Address: "203.0.113.7", PrefixLength: 32, Family: "ipv4"},
			{Address: "2001:db8::1", PrefixLength: 64, Family: "ipv6"},
		}
		if b.expanded {
			info.Addresses = append(info.Addresses, iface.AddrInfo{Address: "203.0.114.1", PrefixLength: 24, Family: "ipv4"})
		}
	default:
		return nil, fmt.Errorf("unknown test interface %s", name)
	}
	if b.overrideAddresses != nil {
		info.Addresses = slices.Clone(b.overrideAddresses)
	}
	return &info, nil
}

func (b *prefixOriginationBackend) GetLink(name string) (*iface.InterfaceInfo, error) {
	info, err := b.GetInterface(name)
	if err != nil {
		return nil, err
	}
	info.Addresses = nil
	info.AddressesRead = false
	return info, nil
}

func (b *prefixOriginationBackend) ListInterfaces() ([]iface.InterfaceInfo, error) {
	var infos []iface.InterfaceInfo
	for _, name := range []string{"lo", "lan0"} {
		info, err := b.GetInterface(name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

var registerPrefixOriginationBackend = sync.OnceValue(func() error {
	return iface.RegisterBackend(prefixOriginationBackendName, func() (iface.Backend, error) {
		return &prefixOriginationBackend{}, nil
	})
})

func installPrefixOriginationBackend(t *testing.T) {
	t.Helper()
	if err := registerPrefixOriginationBackend(); err != nil {
		t.Fatal(err)
	}
	previous := iface.ActiveBackendName()
	if err := iface.LoadBackend(prefixOriginationBackendName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
		if previous != "" {
			if err := iface.LoadBackend(previous); err != nil {
				t.Error(err)
			}
		}
	})
}

// VALIDATES: loopback-only engines retain zero-cost /32 host links while costs
// change, and a changed host waits for MinLSInterval before the maintenance worker
// publishes it. Initial and reload-added enrollment both start that worker.
// PREVENTS: correcting loopback metrics from hiding a missing deferred retry.
func TestLoopbackReloadPublishesHostAfterMinLSInterval(t *testing.T) {
	installOSPFAddressBackend(t)
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000})
	for _, enrollment := range []string{"initial", "reload-added"} {
		t.Run(enrollment, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"100000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"loopback"}}}}}`), nil)
				if err != nil {
					t.Fatal(err)
				}
				eng := newEngine(transport.New(&fakeBackend{}))
				defer eng.shutdown()
				initial := cfg
				if enrollment == "reload-added" {
					initial.Interfaces = nil
				}
				eng.setConfig(initial)
				var host atomic.Uint32
				host.Store(1)
				eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
					topology := eng.lsdbTopology()
					for idx := range topology {
						topology[idx].Address = [4]byte{192, 0, 2, byte(host.Load())}
					}
					return topology
				})
				if err := eng.openInterfaces(); err != nil {
					t.Fatal(err)
				}
				if enrollment == "reload-added" {
					eng.reconcile(cfg)
				}
				eng.originateSelfLSAs()
				synctest.Wait()
				assertLoopbackHostLink(t, eng, cfg.RouterID, [4]byte{192, 0, 2, 1})
				if got := topologyCost(t, eng, "eth0"); got != 100 {
					t.Fatalf("initial topology cost = %d, want 100", got)
				}

				host.Store(9)
				lowered := cfg
				lowered.ReferenceBandwidth = 10000
				eng.reconcile(lowered)
				synctest.Wait()
				if got := topologyCost(t, eng, "eth0"); got != 10 {
					t.Fatalf("reloaded topology cost = %d, want 10", got)
				}
				if got := snapshotByName(t, eng.interfaceSnapshot(), "eth0").Cost; got != 10 {
					t.Fatalf("reloaded CLI cost = %d, want 10", got)
				}
				assertLoopbackHostLink(t, eng, cfg.RouterID, [4]byte{192, 0, 2, 1})
				// sleep(protocol): stop just before the RFC2328 MinLSInterval.
				time.Sleep(5*time.Second - time.Nanosecond)
				synctest.Wait()
				assertLoopbackHostLink(t, eng, cfg.RouterID, [4]byte{192, 0, 2, 1})
				// sleep(protocol): cross the interval and one maintenance tick.
				time.Sleep(time.Second + time.Nanosecond)
				synctest.Wait()
				assertLoopbackHostLink(t, eng, cfg.RouterID, [4]byte{192, 0, 2, 9})
			})
		})
	}
}

func assertLoopbackHostLink(t *testing.T, eng *engine, router types.RouterID, address [4]byte) {
	t.Helper()
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router}
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatal("self Router-LSA was not originated")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatal(err)
	}
	want := []packet.RouterLink{{
		LinkID: types.LinkStateID(address), LinkData: [4]byte{255, 255, 255, 255},
		Type: packet.RouterLinkTypeStub,
	}}
	if !slices.Equal(body.Links, want) {
		t.Fatalf("loopback Router-LSA links = %+v, want zero-cost host link %+v", body.Links, want)
	}
}

// VALIDATES: a passive interface advertises every distinct IPv4 subnet, including
// a non-127 address after Linux's host loopback address, through the real topology.
// PREVENTS: advertising only 127.0.0.0/8 and losing the router's reachable loopback.
func TestPassiveInterfaceOriginatesAllIPv4Prefixes(t *testing.T) {
	installPrefixOriginationBackend(t)
	for _, tc := range []struct {
		name string
		want []packet.RouterLink
	}{
		{name: "lo", want: []packet.RouterLink{
			{LinkID: types.LinkStateID{10, 0, 0, 1}, LinkData: [4]byte{255, 255, 255, 255}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{10, 0, 0, 0}, LinkData: [4]byte{255, 255, 255, 0}, Type: packet.RouterLinkTypeStub, Metric: 17},
		}},
		{name: "lan0", want: []packet.RouterLink{
			{LinkID: types.LinkStateID{192, 0, 2, 0}, LinkData: [4]byte{255, 255, 255, 0}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{192, 0, 2, 0}, LinkData: [4]byte{255, 255, 255, 128}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{198, 51, 100, 0}, LinkData: [4]byte{255, 255, 255, 128}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{203, 0, 113, 7}, LinkData: [4]byte{255, 255, 255, 255}, Type: packet.RouterLinkTypeStub, Metric: 17},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				links := originatedPrefixLinks(t, tc.name, `"passive":"true"`)
				if !slices.Equal(links, tc.want) {
					t.Errorf("Router-LSA links = %+v, want all eligible distinct IPv4 prefixes %+v", links, tc.want)
				}
			})
		})
	}
}

// VALIDATES: explicit loopback network type advertises each usable IPv4 address
// as a zero-cost host stub even when the kernel address has a shorter prefix.
// PREVENTS: treating loopback network type as a connected subnet at interface cost.
func TestLoopbackNetworkOriginatesHostLinks(t *testing.T) {
	installPrefixOriginationBackend(t)
	synctest.Test(t, func(t *testing.T) {
		links := originatedPrefixLinks(t, "lo", `"network-type":"loopback"`)
		want := []packet.RouterLink{
			{LinkID: types.LinkStateID{10, 0, 0, 1}, LinkData: [4]byte{255, 255, 255, 255}, Type: packet.RouterLinkTypeStub},
			{LinkID: types.LinkStateID{10, 0, 0, 9}, LinkData: [4]byte{255, 255, 255, 255}, Type: packet.RouterLinkTypeStub},
		}
		if !slices.Equal(links, want) {
			t.Errorf("loopback Router-LSA links = %+v, want zero-cost host links %+v", links, want)
		}
	})
}

// VALIDATES: an unavailable interface address inventory preserves both existing
// self-LSAs in that area, keeps ABR identity, and allows a healthy area to advance.
// A later successful read advertises every new prefix.
// PREVENTS: a transient read error withdrawing a segment or publishing a partial
// Router-LSA whose omitted secondary prefixes were never observed as absent.
func TestInterfaceInventoryFailurePreservesAreaSelfLSAs(t *testing.T) {
	installPrefixOriginationBackend(t)
	backend, ok := iface.GetBackend().(*prefixOriginationBackend)
	if !ok {
		t.Fatal("prefix origination backend was not loaded")
	}
	synctest.Test(t, func(t *testing.T) {
		cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":"1"},"areas":{"area":{"0":{"area-id":"0"},"1":{"area-id":"1"}}},"interfaces":{"interface":{"lan0":{"area":"0","passive":"true","cost":"17"},"lo":{"area":"1","passive":"true","cost":"17"}}}}}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		eng := newEngine(transport.New(&fakeBackend{}))
		defer eng.shutdown()
		eng.setConfig(cfg)
		// Substitute only the DR election outcome, leaving all live address reads
		// and the Router/Network-LSA builders on the real production path.
		eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
			topology := eng.lsdbTopology()
			for idx := range topology {
				if topology[idx].Name == "lan0" {
					topology[idx].State = ospflsdb.InterfaceStateDR
					topology[idx].DR = cfg.RouterID
					topology[idx].Neighbors = []ospflsdb.NeighborInfo{{RouterID: types.RouterID{10, 0, 0, 2}, Address: netip.MustParseAddr("192.0.2.2"), State: ospflsdb.NeighborStateFull}}
				}
			}
			return topology
		})
		if err := eng.openInterfaces(); err != nil {
			t.Fatal(err)
		}
		eng.originateSelfLSAs()
		synctest.Wait()
		routerKey := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(cfg.RouterID), AdvertisingRouter: cfg.RouterID}
		networkKey := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID{192, 0, 2, 1}, AdvertisingRouter: cfg.RouterID}
		healthyArea := types.AreaID{0, 0, 0, 1}
		beforeRouter, beforeRouterOK := eng.lsdb.LookupLSA(types.BackboneArea, routerKey)
		beforeNetwork, beforeNetworkOK := eng.lsdb.LookupLSA(types.BackboneArea, networkKey)
		beforeHealthy, beforeHealthyOK := eng.lsdb.LookupLSA(healthyArea, routerKey)
		if !beforeRouterOK || !beforeNetworkOK || !beforeHealthyOK {
			t.Fatal("precondition: both areas' Router-LSAs and the DR Network-LSA must exist")
		}

		backend.mu.Lock()
		backend.failName = "lan0"
		backend.expanded = true
		backend.mu.Unlock()
		// sleep(protocol): cross the configured MinLSInterval before requesting
		// an independent origination pass, so rate limiting cannot preserve it.
		time.Sleep(2 * time.Millisecond)
		eng.originateSelfLSAs()
		synctest.Wait()
		afterRouter, ok := eng.lsdb.LookupLSA(types.BackboneArea, routerKey)
		if !ok || afterRouter.Header.Sequence != beforeRouter.Header.Sequence {
			t.Error("failed inventory replaced the area's last complete Router-LSA")
		}
		afterNetwork, ok := eng.lsdb.LookupLSA(types.BackboneArea, networkKey)
		if !ok || afterNetwork.Header.Sequence != beforeNetwork.Header.Sequence || uint16(afterNetwork.Header.Age) == types.MaxAge {
			t.Error("failed inventory replaced or flushed the area's existing Network-LSA")
		}
		afterHealthy, ok := eng.lsdb.LookupLSA(healthyArea, routerKey)
		if !ok || afterHealthy.Header.Sequence == beforeHealthy.Header.Sequence {
			t.Error("a failed area's inventory prevented the healthy area's new prefix from being advertised")
		}
		healthyBody, err := afterHealthy.DecodeRouter()
		if err != nil {
			t.Fatal(err)
		}
		if healthyBody.Flags&packet.RouterFlagB == 0 {
			t.Error("the failed area disappeared from ABR accounting")
		}
		wantHost := packet.RouterLink{LinkID: types.LinkStateID{10, 0, 0, 99}, LinkData: [4]byte{255, 255, 255, 255}, Type: packet.RouterLinkTypeStub, Metric: 17}
		if !slices.Contains(healthyBody.Links, wantHost) {
			t.Error("healthy area did not advertise its newly observed host prefix")
		}

		backend.mu.Lock()
		backend.failName = ""
		backend.mu.Unlock()
		// sleep(protocol): permit the recovered complete snapshot to publish.
		time.Sleep(2 * time.Millisecond)
		eng.originateSelfLSAs()
		synctest.Wait()
		recovered, ok := eng.lsdb.LookupLSA(types.BackboneArea, routerKey)
		if !ok || recovered.Header.Sequence == beforeRouter.Header.Sequence {
			t.Error("successful retry did not replace the retained Router-LSA")
		}
		body, err := recovered.DecodeRouter()
		if err != nil {
			t.Fatal(err)
		}
		for _, prefix := range []packet.RouterLink{
			{LinkID: types.LinkStateID{192, 0, 2, 0}, LinkData: [4]byte{255, 255, 255, 0}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{192, 0, 2, 0}, LinkData: [4]byte{255, 255, 255, 128}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{198, 51, 100, 0}, LinkData: [4]byte{255, 255, 255, 128}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{203, 0, 113, 7}, LinkData: [4]byte{255, 255, 255, 255}, Type: packet.RouterLinkTypeStub, Metric: 17},
			{LinkID: types.LinkStateID{203, 0, 114, 0}, LinkData: [4]byte{255, 255, 255, 0}, Type: packet.RouterLinkTypeStub, Metric: 17},
		} {
			if !slices.Contains(body.Links, prefix) {
				t.Errorf("recovered Router-LSA omitted prefix %+v", prefix)
			}
		}
	})
}

// VALIDATES: a failed lookup between primary-address and mask reads cannot
// replace a valid DR Network-LSA with a zero mask. A subsequent healthy snapshot
// must publish a newer Network-LSA with the new Full neighbor and its valid mask.
// Only the DR election and Full-neighbor outcome are substituted; address reads,
// area admission, body construction, sequencing and installation remain real.
func TestInterfaceInventoryPartialReadKeepsNetworkMask(t *testing.T) {
	installPrefixOriginationBackend(t)
	backend, ok := iface.GetBackend().(*prefixOriginationBackend)
	if !ok {
		t.Fatal("prefix origination backend was not loaded")
	}
	synctest.Test(t, func(t *testing.T) {
		cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":"1"},"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"lan0":{"area":"0","network-type":"broadcast","cost":"17"}}}}}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		eng := newEngine(transport.New(&fakeBackend{}))
		defer eng.shutdown()
		eng.setConfig(cfg)
		var peer atomic.Uint32
		peer.Store(2)
		eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
			topology := eng.lsdbTopology()
			for idx := range topology {
				if topology[idx].Name == "lan0" {
					last := byte(peer.Load())
					topology[idx].State = ospflsdb.InterfaceStateDR
					topology[idx].DR = cfg.RouterID
					topology[idx].Neighbors = []ospflsdb.NeighborInfo{{RouterID: types.RouterID{10, 0, 0, last}, Address: netip.AddrFrom4([4]byte{192, 0, 2, last}), State: ospflsdb.NeighborStateFull}}
				}
			}
			return topology
		})
		if err := eng.openInterfaces(); err != nil {
			t.Fatal(err)
		}
		eng.originateSelfLSAs()
		synctest.Wait()
		key := types.LSAKey{Type: types.LSTypeNetwork, LinkStateID: types.LinkStateID{192, 0, 2, 1}, AdvertisingRouter: cfg.RouterID}
		before, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
		if !ok {
			t.Fatal("precondition: DR Network-LSA was not originated")
		}
		beforeBody, err := before.DecodeNetwork()
		if err != nil {
			t.Fatal(err)
		}
		wantMask := [4]byte{255, 255, 255, 0}
		if beforeBody.NetworkMask != wantMask {
			t.Fatalf("precondition: Network-LSA mask = %v, want %v", beforeBody.NetworkMask, wantMask)
		}
		if !slices.Equal(beforeBody.AttachedRouters, []types.RouterID{cfg.RouterID, {10, 0, 0, 2}}) {
			t.Fatalf("precondition: attached routers = %v", beforeBody.AttachedRouters)
		}

		backend.mu.Lock()
		backend.failNthName = "lan0"
		backend.failNthRead = 2
		backend.namedReads = 0
		backend.faultRead = false
		backend.mu.Unlock()
		// sleep(protocol): cross MinLSInterval so an invalid changed body cannot
		// be hidden by the originator's one-millisecond rate limit.
		time.Sleep(2 * time.Millisecond)
		eng.originateSelfLSAs()
		synctest.Wait()
		backend.mu.Lock()
		reads, faultRead := backend.namedReads, backend.faultRead
		backend.failNthRead = 0
		backend.mu.Unlock()
		t.Logf("fault pass: %d named inventory lookups, second-read fault triggered=%t", reads, faultRead)
		if reads == 0 {
			t.Fatal("origination did not read the interface inventory")
		}
		// A coherent single-read implementation has no second IPv4 lookup to
		// fail. If it performs further lookups, the scripted fault must run.
		if reads > 1 {
			if !faultRead {
				t.Fatal("precondition: the second inventory lookup did not fail")
			}
		}
		afterFault, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
		if !ok {
			t.Fatal("partial read removed the existing DR Network-LSA")
		}
		afterBody, err := afterFault.DecodeNetwork()
		if err != nil {
			t.Fatal(err)
		}
		if afterBody.NetworkMask != wantMask {
			t.Errorf("partial read published Network-LSA mask %v, want retained valid mask %v", afterBody.NetworkMask, wantMask)
		}
		if uint16(afterFault.Header.Age) == types.MaxAge {
			t.Error("partial read flushed the existing DR Network-LSA")
		}

		peer.Store(3)
		// sleep(protocol): permit the healthy changed neighbor set to produce
		// a new Network-LSA instead of merely retaining the original body.
		time.Sleep(2 * time.Millisecond)
		eng.originateSelfLSAs()
		synctest.Wait()
		recovered, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
		if !ok {
			t.Fatal("healthy retry originated no DR Network-LSA")
		}
		recoveredBody, err := recovered.DecodeNetwork()
		if err != nil {
			t.Fatal(err)
		}
		if recoveredBody.NetworkMask != wantMask {
			t.Errorf("healthy retry Network-LSA mask = %v, want %v", recoveredBody.NetworkMask, wantMask)
		}
		if !slices.Equal(recoveredBody.AttachedRouters, []types.RouterID{cfg.RouterID, {10, 0, 0, 3}}) {
			t.Errorf("healthy retry attached routers = %v, want self and new Full neighbor", recoveredBody.AttachedRouters)
		}
		if !recovered.Header.Sequence.NewerThan(afterFault.Header.Sequence) {
			t.Errorf("healthy retry sequence %s is not newer than %s", recovered.Header.Sequence, afterFault.Header.Sequence)
		}
	})
}

// VALIDATES: one live read supplies a paired primary address/mask and exact
// secondary prefixes, retaining host bits and same-address/different-mask entries.
// Malformed addresses, a non-IPv4 literal and invalid prefix boundaries cannot
// displace the valid primary or invent reachability; its exact duplicate is omitted.
func TestInterfaceIPv4InventoryPairsAndFiltersAddresses(t *testing.T) {
	installPrefixOriginationBackend(t)
	backend, ok := iface.GetBackend().(*prefixOriginationBackend)
	if !ok {
		t.Fatal("prefix origination backend was not loaded")
	}
	primary := iface.AddrInfo{Address: "192.0.2.129", PrefixLength: 24, Family: "ipv4"}
	secondary := iface.AddrInfo{Address: "198.51.100.65", PrefixLength: 25, Family: "ipv4"}
	wantAddress := [4]byte{192, 0, 2, 129}
	wantMask := [4]byte{255, 255, 255, 0}
	wantSecondary := []netip.Prefix{netip.MustParsePrefix("198.51.100.65/25")}
	for _, test := range []struct {
		name        string
		addresses   []iface.AddrInfo
		primary     [4]byte
		mask        [4]byte
		secondaries []netip.Prefix
	}{
		{name: "paired-primary-and-host-bits", addresses: []iface.AddrInfo{primary, secondary}, primary: wantAddress, mask: wantMask, secondaries: wantSecondary},
		{name: "malformed-address-before-primary", addresses: []iface.AddrInfo{{Address: "192.0.2.999", PrefixLength: 24, Family: "ipv4"}, primary, secondary}, primary: wantAddress, mask: wantMask, secondaries: wantSecondary},
		{name: "ipv6-literal-declared-ipv4", addresses: []iface.AddrInfo{{Address: "2001:db8::1", PrefixLength: 24, Family: "ipv4"}, primary, secondary}, primary: wantAddress, mask: wantMask, secondaries: wantSecondary},
		{name: "prefix-below-zero", addresses: []iface.AddrInfo{{Address: "203.0.113.5", PrefixLength: -1, Family: "ipv4"}, primary, secondary}, primary: wantAddress, mask: wantMask, secondaries: wantSecondary},
		{name: "prefix-above-thirty-two", addresses: []iface.AddrInfo{{Address: "203.0.113.5", PrefixLength: 33, Family: "ipv4"}, primary, secondary}, primary: wantAddress, mask: wantMask, secondaries: wantSecondary},
		{name: "prefix-zero-is-valid", addresses: []iface.AddrInfo{{Address: "192.0.2.129", PrefixLength: 0, Family: "ipv4"}, secondary}, primary: wantAddress, mask: [4]byte{}, secondaries: wantSecondary},
		{name: "prefix-thirty-two-is-valid", addresses: []iface.AddrInfo{{Address: "192.0.2.129", PrefixLength: 32, Family: "ipv4"}, secondary}, primary: wantAddress, mask: [4]byte{255, 255, 255, 255}, secondaries: wantSecondary},
		{name: "exact-primary-duplicate", addresses: []iface.AddrInfo{primary, primary, secondary}, primary: wantAddress, mask: wantMask, secondaries: wantSecondary},
		{name: "same-primary-address-different-mask", addresses: []iface.AddrInfo{primary, {Address: "192.0.2.129", PrefixLength: 25, Family: "ipv4"}, secondary}, primary: wantAddress, mask: wantMask, secondaries: []netip.Prefix{netip.MustParsePrefix("192.0.2.129/25"), netip.MustParsePrefix("198.51.100.65/25")}},
		{name: "host127-primary-is-preserved", addresses: []iface.AddrInfo{{Address: "127.0.0.1", PrefixLength: 8, Family: "ipv4"}, {Address: "10.0.0.9", PrefixLength: 24, Family: "ipv4"}}, primary: [4]byte{127, 0, 0, 1}, mask: [4]byte{255, 0, 0, 0}, secondaries: []netip.Prefix{netip.MustParsePrefix("10.0.0.9/24")}},
		{name: "known-empty-inventory", addresses: []iface.AddrInfo{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend.mu.Lock()
			backend.overrideAddresses = slices.Clone(test.addresses)
			backend.failNthName = "lan0"
			backend.failNthRead = 2
			backend.namedReads = 0
			backend.faultRead = false
			backend.mu.Unlock()
			got, err := readInterfaceIPv4Inventory("lan0")
			if err != nil {
				t.Fatal(err)
			}
			if got.address != test.primary {
				t.Errorf("primary address = %v, want %v", got.address, test.primary)
			}
			if got.mask != test.mask {
				t.Errorf("primary mask = %v, want %v", got.mask, test.mask)
			}
			if !slices.Equal(got.secondaryPrefixes, test.secondaries) {
				t.Errorf("secondary prefixes = %v, want %v", got.secondaryPrefixes, test.secondaries)
			}
			backend.mu.Lock()
			reads := backend.namedReads
			backend.mu.Unlock()
			if reads != 1 {
				t.Errorf("inventory backend reads = %d, want exactly one", reads)
			}
		})
	}
}

func originatedPrefixLinks(t *testing.T, name, options string) []packet.RouterLink {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"`+name+`":{"area":"0","cost":"17",`+options+`}}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	defer eng.shutdown()
	eng.setConfig(cfg)
	if err := eng.openInterfaces(); err != nil {
		t.Fatal(err)
	}
	eng.originateSelfLSAs()
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(cfg.RouterID), AdvertisingRouter: cfg.RouterID}
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		t.Fatal("self Router-LSA was not originated")
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatal(err)
	}
	return body.Links
}

// VALIDATES: interface inventory failures and recovery send one actionable
// warning and its matching clear through the existing SDK DirectBridge.
// PREVENTS: raising on a forked plugin's private report bus, or sending a
// report for every packet-driven topology lookup while a read remains failed.
// The daemon's report-issue handler and bus admission have their own existing
// tests in internal/component/plugin/server/dispatch_report_test.go.
func TestInterfaceInventoryReportsThroughSDKBridge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		installPrefixOriginationBackend(t)
		backend, ok := iface.GetBackend().(*prefixOriginationBackend)
		if !ok {
			t.Fatal("prefix origination backend was not loaded")
		}
		pluginEnd, engineEnd := net.Pipe()
		bridge := rpc.NewDirectBridge()
		requests := make(chan rpc.ReportIssueInput, 32)
		bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
			if method != rpc.MethodReportIssue {
				return nil, fmt.Errorf("unexpected SDK method %s", method)
			}
			var in rpc.ReportIssueInput
			if err := json.Unmarshal(params, &in); err != nil {
				return nil, err
			}
			requests <- in
			return json.RawMessage(`{}`), nil
		})
		bridge.SetReady()
		client := sdk.NewWithConn(Namespace, rpc.NewBridgedConn(pluginEnd, bridge))
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Error(err)
			}
			if err := engineEnd.Close(); err != nil {
				t.Error(err)
			}
		})
		cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"lan0":{"area":"0","passive":"true"}}}}}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		eng := newEngine(transport.New(&fakeBackend{}))
		defer eng.shutdown()
		eng.setConfig(cfg)
		if err := eng.openInterfaces(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		// Bootstrap persistence has its own tests. Attach the real SDK only after
		// enrollment so this bridge remains strict about the report RPC it proves.
		eng.state = client
		backend.mu.Lock()
		backend.failName = "lan0"
		backend.mu.Unlock()
		eng.lsdbTopology()
		eng.lsdbTopology()
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		const code = "doctor-ospf-interface-addresses-unavailable"
		const fix = "ze show interface name lan0 detail"
		var raised rpc.ReportIssueInput
		select {
		case raised = <-requests:
		default:
			t.Fatal("failed inventory sent no warning through the SDK report-issue bridge")
		}
		if raised.Action != rpc.ReportRaise || raised.Severity != rpc.ReportWarning || raised.Code != code || raised.Subject != "lan0 instance 0" || raised.Fix != fix {
			t.Errorf("SDK warning = %+v, want warning raise under %s for lan0 instance 0 with fix %s", raised, code, fix)
		}
		for _, part := range []string{"OSPF cannot read the IPv4 addresses of interface lan0", "last complete Router and Network LSAs", "test interface lan0 address inventory unavailable", "fix: " + fix} {
			if !strings.Contains(raised.Message, part) {
				t.Errorf("SDK warning message %q omits %q", raised.Message, part)
			}
		}
		select {
		case extra := <-requests:
			t.Errorf("repeated failed topology read sent another SDK warning: %+v", extra)
		default:
		}
		backend.mu.Lock()
		backend.failName = ""
		backend.mu.Unlock()
		eng.lsdbTopology()
		assertNoInventoryReport(t, requests)
		inventoryMaintenanceTick()
		select {
		case cleared := <-requests:
			want := rpc.ReportIssueInput{Action: rpc.ReportClear, Code: code, Subject: raised.Subject}
			if cleared != want {
				t.Errorf("SDK recovery request = %+v, want matching clear %+v", cleared, want)
			}
		default:
			t.Error("successful inventory read sent no matching SDK warning clear")
		}
	})
}
