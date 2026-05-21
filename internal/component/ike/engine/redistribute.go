// Design: docs/architecture/core-design.md -- redistribute source registration
package engine

import (
	"log/slog"
	"net"
	"net/netip"
	"sync"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var redistOnce sync.Once

func registerIPsecRedistSources() {
	redistOnce.Do(func() {
		err := configredist.RegisterSource(configredist.RouteSource{
			Name:        "ipsec",
			Protocol:    "ipsec",
			Description: "tunnel routes from IPsec Child SAs",
		})
		if err != nil {
			slog.Error("BUG: failed to register ipsec redistribute source", "err", err)
		}
	})
}

var ipsecProtocolID = redistevents.RegisterProtocol(Namespace)

var _ = registerIPsecProducer()

func registerIPsecProducer() bool {
	redistevents.RegisterProducer(ipsecProtocolID)
	return true
}

var ipsecRouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)

func emitRouteAdd(bus ze.EventBus, tsRemote *net.IPNet, log *slog.Logger) {
	if bus == nil || tsRemote == nil {
		return
	}
	prefix, ok := netIPNetToPrefix(tsRemote)
	if !ok {
		return
	}
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
		Action: redistevents.ActionAdd,
		Prefix: prefix,
	})
	if _, err := ipsecRouteChange.Emit(bus, batch); err != nil {
		log.Debug("ipsec: emit route-change add failed", "error", err)
	}
	redistevents.ReleaseBatch(batch)
}

func emitRouteRemove(bus ze.EventBus, tsRemote *net.IPNet, log *slog.Logger) {
	if bus == nil || tsRemote == nil {
		return
	}
	prefix, ok := netIPNetToPrefix(tsRemote)
	if !ok {
		return
	}
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
		Action: redistevents.ActionRemove,
		Prefix: prefix,
	})
	if _, err := ipsecRouteChange.Emit(bus, batch); err != nil {
		log.Debug("ipsec: emit route-change remove failed", "error", err)
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
