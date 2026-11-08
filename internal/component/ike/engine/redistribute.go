// Design: docs/architecture/core-design.md -- redistribute source registration
package engine

import (
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/ze"
)

var redistOnce sync.Once

func registerIPsecRedistSources() {
	redistOnce.Do(func() {
		err := configredist.RegisterSource(configredist.RouteSource{
			Name:        ipsecName,
			Protocol:    ipsecName,
			Description: "tunnel routes from IPsec Child SAs",
		})
		if err != nil {
			slog.Error("BUG: failed to register ipsec redistribute source", "err", err)
		}
	})
}

// ipsecName is the one name this engine registers under: the redistribute
// source, its protocol, and the kernel capability subsystem.
const ipsecName = "ipsec"

var ipsecProtocolID = redistevents.RegisterProtocol(ipsecName)

var _ = registerIPsecProducer()

func registerIPsecProducer() bool {
	redistevents.RegisterProducer(ipsecProtocolID)
	return true
}

var ipsecRouteChange = events.Register[*redistevents.RouteChangeBatch](ipsecName, redistevents.EventType)

// emitRouteAdd announces every remote traffic selector child installs a policy for, one
// route-change batch per prefix. A Child SA can negotiate several selector pairs (IPv4
// first and IPv6 second on a route-based xfrm interface), and TSRemote holds only the
// first, so the prefixes come from childRemotePrefixes: announcing TSRemote alone left
// every later pair's prefix unknown to the redistribute consumers. A batch carries one
// AFI, so each prefix travels in its own. emitRouteRemove withdraws the same set.
func emitRouteAdd(bus ze.EventBus, child *ChildSA, log *slog.Logger) {
	if bus == nil {
		log.Warn("ipsec: emit route-change skipped: no event bus")
		return
	}
	if child == nil {
		log.Warn("ipsec: emit route-change skipped: nil Child SA")
		return
	}
	prefixes := childRemotePrefixes(child)
	if len(prefixes) == 0 {
		log.Warn("ipsec: emit route-change skipped: Child SA has no remote traffic selector")
		return
	}
	for _, prefix := range prefixes {
		log.Debug("ipsec: emitting route-change add", "prefix", prefix.String())
		emitRouteChange(bus, redistevents.ActionAdd, prefix, log)
	}
}

// emitRouteRemove withdraws every prefix emitRouteAdd announced for child. Both read
// childRemotePrefixes, so the withdrawal names exactly the announced set; a Child SA's
// selectors never change while it lives (a rekey builds a new ChildSA, and rekeyTunnelRoutes moves the routes).
func emitRouteRemove(bus ze.EventBus, child *ChildSA, log *slog.Logger) {
	if bus == nil {
		return
	}
	if child == nil {
		return
	}
	for _, prefix := range childRemotePrefixes(child) {
		emitRouteChange(bus, redistevents.ActionRemove, prefix, log)
	}
}

// rekeyTunnelRoutes moves peer's tunnel routes, on the route-change bus and in the
// tunnel-source guard, from retired to the successor a Child SA rekey installed. A rekey
// can narrow or change the selector set (newRekeyedChild adopts what the exchange
// negotiated), so it announces only the prefixes successor adds and withdraws only the
// ones it dropped. A prefix both carry is left alone: withdrawing and re-announcing it
// would flap the route at every rekey. The successor's prefixes are announced before the
// retired ones are withdrawn, make-before-break as the SA swap itself is.
func rekeyTunnelRoutes(bus ze.EventBus, peer string, retired, successor *ChildSA, log *slog.Logger) {
	if retired == nil {
		emitRouteAdd(bus, successor, log)
		guardTunnelRoute(true, peer, successor, log)
		return
	}
	before := childRemotePrefixes(retired)
	after := childRemotePrefixes(successor)
	added := prefixesAbsentFrom(after, before)
	dropped := prefixesAbsentFrom(before, after)
	if bus == nil {
		log.Warn("ipsec: rekey route-change skipped: no event bus", "peer", peer)
	} else {
		for _, prefix := range added {
			emitRouteChange(bus, redistevents.ActionAdd, prefix, log)
		}
		for _, prefix := range dropped {
			emitRouteChange(bus, redistevents.ActionRemove, prefix, log)
		}
	}
	// A guard entry is keyed by the interface too, so a successor on another interface
	// (never the case today: newRekeyedChild inherits IfID) moves the whole set.
	if retired.IfID != successor.IfID {
		guardTunnelPrefixes(true, peer, successor.IfID, after, log)
		guardTunnelPrefixes(false, peer, retired.IfID, before, log)
		return
	}
	guardTunnelPrefixes(true, peer, successor.IfID, added, log)
	guardTunnelPrefixes(false, peer, retired.IfID, dropped, log)
}

// prefixesAbsentFrom answers the prefixes of set that other does not hold, in set's order.
func prefixesAbsentFrom(set, other []netip.Prefix) []netip.Prefix {
	var out []netip.Prefix
	for _, prefix := range set {
		if slices.Contains(other, prefix) {
			continue
		}
		out = append(out, prefix)
	}
	return out
}

// emitRouteChange publishes one route-change batch carrying prefix under action.
func emitRouteChange(bus ze.EventBus, action redistevents.RouteAction, prefix netip.Prefix, log *slog.Logger) {
	batch := redistevents.AcquireBatch()
	batch.Protocol = ipsecProtocolID
	if prefix.Addr().Is4() {
		batch.AFI = 1
		batch.SAFI = 1
	} else {
		batch.AFI = 2
		batch.SAFI = 1
	}
	batch.Entries = append(batch.Entries, redistevents.RouteChangeEntry{
		Action: action,
		Prefix: prefix,
	})
	n, err := ipsecRouteChange.Emit(bus, batch)
	if err != nil {
		log.Warn("ipsec: emit route-change failed", "action", action, "prefix", prefix.String(), "error", err)
	} else {
		log.Debug("ipsec: route-change delivered", "action", action, "prefix", prefix.String(), "subscribers", n)
	}
	redistevents.ReleaseBatch(batch)
}

func netIPNetToPrefix(ipNet *net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(ipNet.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, _ := ipNet.Mask.Size()
	return netip.PrefixFrom(addr.Unmap(), ones), true
}
