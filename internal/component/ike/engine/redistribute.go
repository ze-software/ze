// Design: docs/architecture/core-design.md -- redistribute source registration
package engine

import (
	"log/slog"
	"net"
	"net/netip"
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

func emitRouteAdd(bus ze.EventBus, tsRemote *net.IPNet, log *slog.Logger) {
	if bus == nil {
		log.Warn("ipsec: emit route-change skipped: no event bus")
		return
	}
	if tsRemote == nil {
		log.Warn("ipsec: emit route-change skipped: nil tsRemote")
		return
	}
	log.Debug("ipsec: emitting route-change add", "prefix", tsRemote.String())
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
	n, err := ipsecRouteChange.Emit(bus, batch)
	if err != nil {
		log.Warn("ipsec: emit route-change add failed", "error", err)
	} else {
		log.Debug("ipsec: route-change add delivered", "prefix", tsRemote.String(), "subscribers", n)
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
