// Design: docs/architecture/core-design.md -- connected route redistribution
// Related: events/events.go -- typed EventBus handle for route-change

package connected

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
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
	bus ze.EventBus

	mu       sync.Mutex
	prefixes map[netip.Prefix]int
}

func newRouteObserver(bus ze.EventBus) *routeObserver {
	return &routeObserver{
		bus:      bus,
		prefixes: make(map[netip.Prefix]int),
	}
}

func (o *routeObserver) handleAddrAdded(payload any) {
	p, ok := o.parsePayload(payload)
	if !ok {
		return
	}
	prefix, ok := o.toNetworkPrefix(p)
	if !ok {
		return
	}
	o.mu.Lock()
	o.prefixes[prefix]++
	count := o.prefixes[prefix]
	o.mu.Unlock()
	if count == 1 {
		o.emit(redistevents.ActionAdd, prefix)
	}
}

func (o *routeObserver) handleAddrRemoved(payload any) {
	p, ok := o.parsePayload(payload)
	if !ok {
		return
	}
	prefix, ok := o.toNetworkPrefix(p)
	if !ok {
		return
	}
	o.mu.Lock()
	o.prefixes[prefix]--
	count := o.prefixes[prefix]
	if count <= 0 {
		delete(o.prefixes, prefix)
	}
	o.mu.Unlock()
	if count <= 0 {
		o.emit(redistevents.ActionRemove, prefix)
	}
}

func (o *routeObserver) parsePayload(payload any) (addrPayload, bool) {
	var p addrPayload
	data, ok := payload.([]byte)
	if !ok {
		if s, ok2 := payload.(string); ok2 {
			data = []byte(s)
		} else {
			return p, false
		}
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, false
	}
	return p, true
}

func (o *routeObserver) toNetworkPrefix(p addrPayload) (netip.Prefix, bool) {
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap()
	if p.PrefixLength <= 0 {
		return netip.Prefix{}, false
	}
	prefix, err := addr.Prefix(p.PrefixLength)
	if err != nil {
		return netip.Prefix{}, false
	}
	return prefix, true
}

func (o *routeObserver) emit(action redistevents.RouteAction, prefix netip.Prefix) {
	o.emitID(action, prefix, 0)
}

// emitID emits a single-prefix batch tagged with replayID (0 for the normal
// incremental path; nonzero echoes a redistribute ReplayRequest so the
// orchestrator can replay to a newly-established peer).
func (o *routeObserver) emitID(action redistevents.RouteAction, prefix netip.Prefix, replayID uint64) {
	if o.bus == nil {
		return
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
	if _, err := connectedevents.RouteChange.Emit(o.bus, b); err != nil {
		logger().Warn("connected: route-change emit failed", "error", err)
	}
}

// reemitAll re-emits every currently-connected prefix as an add tagged with
// replayID, so the redistribute orchestrator can replay them to a peer that
// establishes after the original emit. Reflects the CURRENT live set (a prefix
// removed before the peer joined is simply absent). A zero replayID is a no-op
// (the orchestrator only allocates nonzero tokens).
func (o *routeObserver) reemitAll(replayID uint64) {
	if o.bus == nil || replayID == 0 {
		return
	}
	o.mu.Lock()
	prefixes := make([]netip.Prefix, 0, len(o.prefixes))
	for p, count := range o.prefixes {
		if count > 0 {
			prefixes = append(prefixes, p)
		}
	}
	o.mu.Unlock()
	for _, p := range prefixes {
		o.emitID(redistevents.ActionAdd, p, replayID)
	}
}

func runConnectedPlugin(conn net.Conn) int {
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	registerConnectedSources()

	bus := getEventBus()
	obs := newRouteObserver(bus)

	if bus != nil {
		unsub1 := bus.Subscribe("interface", "addr-added", obs.handleAddrAdded)
		unsub2 := bus.Subscribe("interface", "addr-removed", obs.handleAddrRemoved)
		// Redistribute late-join replay: on a ReplayRequest re-emit the current
		// connected-route set tagged with the echoed ReplayID so a peer that
		// established after injection receives them (spec-redistribute-late-join-replay).
		unsub3 := redistevents.ReplayRequestEvent.Subscribe(bus, func(r *redistevents.ReplayRequest) {
			obs.reemitAll(r.ReplayID)
		})
		defer unsub1()
		defer unsub2()
		defer unsub3()
	}

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{pluginName},
	}); err != nil {
		logger().Error("connected plugin failed", "error", err)
		return 1
	}
	return 0
}
