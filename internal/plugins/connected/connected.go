// Design: docs/architecture/core-design.md -- connected route redistribution
// Related: events/events.go -- typed EventBus handle for route-change

package connected

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routeinstall"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

const pluginName = "connected"

var sourcesOnce sync.Once

func registerConnectedSources() {
	sourcesOnce.Do(func() {
		_ = redistribute.RegisterSource(redistribute.RouteSource{
			Name:        pluginName,
			Protocol:    pluginName,
			Description: "directly connected interface routes",
		})
	})
}

type addrPayload struct {
	Name         string `json:"name"`
	Unit         int    `json:"unit"`
	Index        int    `json:"index"`
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix-length"`
	Family       string `json:"family"`
	Managed      bool   `json:"managed"`
}

type routeObserver struct {
	// publish consumes the batch synchronously. It MUST finish encoding or
	// dispatching before returning: emitID returns the batch to its pool.
	// Nil disables redistribution, as a nil bus does in newRouteObserver.
	publish func(*redistevents.RouteChangeBatch) (int, error)
	// reportReplay reports a complete replay's outcome under emitMu. Nil
	// disables reporting for an observer with no SDK connection.
	reportReplay func(error) error

	mu sync.Mutex
	// emitMu orders each complete address change, replay and retirement.
	// Retirement drains accepted publications before its withdrawals; later
	// events cannot revive the retired observer. An address set cannot change
	// ahead of an earlier publication for the same prefix either.
	// Address handlers MUST hold it through insertPath/removePath and emit.
	emitMu  sync.RWMutex
	retired bool // guarded by emitMu; set only after all stop withdrawals succeed.
	// prefixes holds, for each connected prefix, the addresses that cover it.
	// It is a SET rather than a count of events, because the kernel announces
	// one address more than once: the iface monitor's startup dump can overlap
	// a live RTM_NEWADDR for the same address, and an IPv6 address is
	// re-announced each time its lifetimes change. Counting those would leave a
	// prefix in the Loc-RIB after its last address went.
	prefixes map[netip.Prefix]map[connectedAddr]struct{}

	// loc and remote are where a connected prefix is published: the shared
	// Loc-RIB in-process, or the engine over RPC when connected runs forked.
	// See locrib.go.
	loc    *locrib.RIB
	remote routeSink
}

// connectedAddr names one address on one interface unit, the identity the
// prefix set counts.
type connectedAddr struct {
	name    string
	unit    int
	address netip.Addr
}

func newRouteObserver(bus ze.EventBus) *routeObserver {
	o := &routeObserver{
		prefixes: make(map[netip.Prefix]map[connectedAddr]struct{}),
	}
	if bus != nil {
		o.publish = func(batch *redistevents.RouteChangeBatch) (int, error) {
			return connectedevents.RouteChange.Emit(bus, batch)
		}
	}
	return o
}

func (o *routeObserver) handleAddrAdded(payload any) {
	p, ok := o.parsePayload(payload)
	if !ok {
		return
	}
	key, prefix, ok := o.toNetworkPrefix(p)
	if !ok {
		return
	}
	o.emitMu.Lock()
	defer o.emitMu.Unlock()
	if o.retired {
		return
	}
	o.mu.Lock()
	covering := o.prefixes[prefix]
	if covering == nil {
		covering = make(map[connectedAddr]struct{})
		o.prefixes[prefix] = covering
	}
	_, known := covering[key]
	if known {
		o.mu.Unlock()
		return
	}
	newDevice := !coversDevice(covering, key)
	covering[key] = struct{}{}
	first := len(covering) == 1
	o.mu.Unlock()
	if newDevice {
		// One prefix path carries every covering device. Another address on
		// the same device changes neither that path nor redistribution.
		o.insertPath(prefix, covering)
	}
	if first {
		// The prefix becomes visible to route arbitration and to recursive
		// next-hop resolution the first time an address covers it, and stays
		// visible until the last one goes. Redistribution follows that prefix
		// lifetime, independently of changes to its covering devices.
		o.emit(redistevents.ActionAdd, prefix)
	}
}

func (o *routeObserver) handleAddrRemoved(payload any) {
	p, ok := o.parsePayload(payload)
	if !ok {
		return
	}
	key, prefix, ok := o.toNetworkPrefix(p)
	if !ok {
		return
	}
	o.emitMu.Lock()
	defer o.emitMu.Unlock()
	if o.retired {
		return
	}
	o.mu.Lock()
	covering, held := o.prefixes[prefix]
	if !held {
		// A removal for a prefix no address covers withdraws nothing: the
		// prefix was never inserted, or its last address already went.
		o.mu.Unlock()
		return
	}
	_, known := covering[key]
	if !known {
		o.mu.Unlock()
		return
	}
	delete(covering, key)
	last := len(covering) == 0
	if last {
		delete(o.prefixes, prefix)
	}
	deviceRemoved := !coversDevice(covering, key)
	o.mu.Unlock()
	if last {
		o.removePath(prefix)
		o.emit(redistevents.ActionRemove, prefix)
		return
	}
	if deviceRemoved {
		o.insertPath(prefix, covering)
	}
}

// coversDevice reports whether an address still covers the named device.
func coversDevice(covering map[connectedAddr]struct{}, key connectedAddr) bool {
	for address := range covering {
		if address.name != key.name {
			continue
		}
		if address.unit == key.unit {
			return true
		}
	}
	return false
}

func (o *routeObserver) parsePayload(payload any) (addrPayload, bool) {
	var p addrPayload
	data, ok := payload.([]byte)
	if !ok {
		s, ok2 := payload.(string)
		if !ok2 {
			return p, false
		}

		data = []byte(s)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, false
	}
	return p, true
}

// toNetworkPrefix answers the address an event names, as the key the prefix
// set counts, and the connected prefix that address covers.
func (o *routeObserver) toNetworkPrefix(p addrPayload) (connectedAddr, netip.Prefix, bool) {
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		return connectedAddr{}, netip.Prefix{}, false
	}
	addr = addr.Unmap()
	if p.PrefixLength <= 0 {
		return connectedAddr{}, netip.Prefix{}, false
	}
	prefix, err := addr.Prefix(p.PrefixLength)
	if err != nil {
		return connectedAddr{}, netip.Prefix{}, false
	}
	return connectedAddr{name: p.Name, unit: p.Unit, address: addr}, prefix, true
}

func (o *routeObserver) emit(action redistevents.RouteAction, prefix netip.Prefix) {
	if err := o.emitID(action, prefix, 0); err != nil {
		logger().Warn("connected: route-change emit failed", "error", err)
	}
}

// emitID emits a single-prefix batch tagged with replayID (0 for the normal
// incremental path; nonzero echoes a redistribute ReplayRequest so the
// orchestrator can replay to a newly-established peer or to a destination
// consumer that registered after this producer emitted).
func (o *routeObserver) emitID(action redistevents.RouteAction, prefix netip.Prefix, replayID uint64) error {
	if o.publish == nil {
		return nil
	}
	// The prefix is in the Loc-RIB whatever its address, because the kernel
	// holds its route; it only never reaches the redistribution feed.
	if !redistributable(prefix) {
		return nil
	}
	var fam family.Family
	if prefix.Addr().Is4() {
		fam = family.IPv4Unicast
	} else {
		fam = family.IPv6Unicast
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = connectedevents.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.ReplayID = replayID
	b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
		Action: action,
		Prefix: prefix,
	})
	_, err := o.publish(b)
	return err
}

// redistributable reports whether a connected prefix may be offered to the
// redistribute orchestrator. It refuses the set FRR's zebra never hands a
// redistribute client, zebra_check_addr in zebra/zebra_rib.c, which
// zebra_redistribute_add and redistribute_update (zebra/redistribute.c) call
// before they send a route:
//
//	if (IPV4_NET127(addr) || IN_CLASSD(addr) ||
//	    (IPV4_LINKLOCAL(addr) && !IPV4_CLASS_E(addr)))
//		return 0;
//	...
//	if (IN6_IS_ADDR_LOOPBACK(&p->u.prefix6))
//		return 0;
//	if (IN6_IS_ADDR_LINKLOCAL(&p->u.prefix6))
//		return 0;
//
// It also refuses the zero network 0/8, which BIRD's ip4_classify (lib/ip.c)
// answers IADDR_INVALID, so BIRD never learns such an interface address at
// all. A prefix is judged by its network address, as zebra judges rn->p.
func redistributable(prefix netip.Prefix) bool {
	addr := prefix.Addr()
	if addr.IsLoopback() {
		return false
	}
	if addr.IsLinkLocalUnicast() {
		return false
	}
	if addr.IsMulticast() {
		return false
	}
	if addr.Is4() && addr.As4()[0] == 0 {
		return false
	}
	return true
}

// reemitAll re-emits every currently-connected prefix as an add tagged with
// replayID, so the redistribute orchestrator can replay them to a peer that
// establishes after the original emit. Reflects the CURRENT live set (a prefix
// removed before the peer joined is simply absent). A zero replayID is a no-op
// (the orchestrator only allocates nonzero tokens).
func (o *routeObserver) reemitAll(replayID uint64) error {
	if o.publish == nil {
		return nil
	}
	if replayID == 0 {
		return nil
	}
	o.emitMu.Lock()
	defer o.emitMu.Unlock()
	if o.retired {
		return nil
	}
	o.mu.Lock()
	prefixes := make([]netip.Prefix, 0, len(o.prefixes))
	for p, covering := range o.prefixes {
		if len(covering) > 0 {
			prefixes = append(prefixes, p)
		}
	}
	o.mu.Unlock()
	var replayFailure error
	for _, p := range prefixes {
		if err := o.emitID(redistevents.ActionAdd, p, replayID); err != nil {
			logger().Warn("connected: route-change replay failed", "error", err,
				"prefix", p, "code", diagnostic.CodeDoctorPluginPanic, "fix", connectedReplayFix)
			if replayFailure == nil {
				replayFailure = fmt.Errorf("prefix %s: %w", p, err)
			}
		}
	}
	if o.reportReplay != nil {
		return o.reportReplay(replayFailure)
	}
	return replayFailure
}

func runConnectedPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	registerConnectedSources()

	obs := newRouteObserver(nil)
	// Connected prefixes are published into the shared Loc-RIB. In-process that
	// is locrib.Default(); forked, it answers nil and the operations travel to
	// the engine over the route-install RPC, which rebuilds the Path in the
	// engine's own Loc-RIB.
	loc := locrib.Default()
	var remote routeSink
	if loc == nil {
		remote = routeinstall.New(context.Background(), p)
	}
	obs.setLocRIB(loc, remote)
	defer obs.withdrawLocalPaths()

	ctx := context.Background()
	followInterfaceAddresses(ctx, p, obs)

	// The connected container is empty: the plugin keeps WantsConfig so the
	// root autoloads it and counts it as a consumer, and a reload has nothing
	// for it to apply.
	p.DeclareAppliesNothing("its config container holds no leaf")

	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{pluginName},
	}); err != nil {
		logger().Error("connected plugin failed", "error", err)
		return 1
	}
	return 0
}
