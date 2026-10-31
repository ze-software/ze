//go:build linux

package ifacenetlink

import (
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// routerEvents counts the router-discovered and router-lost events the bus
// recorded.
func routerEvents(bus *collectingEventBus) (discovered, lost int) {
	for _, event := range bus.snapshot() {
		switch event.EventType {
		case ifaceevents.EventRouterDiscovered:
			discovered++
		case ifaceevents.EventRouterLost:
			lost++
		}
	}
	return discovered, lost
}

// TestNeighbourUpdateRouterEvents drives the monitor's handleNeighUpdate with
// the neighbor updates the neighbor table watch also reads: none of the
// updates without NTF_ROUTER, a named Link-Local PERMANENT entry included,
// emits a router event, and a Link-Local that gains NTF_ROUTER emits one
// discovered event, then one lost event when it is deleted.
func TestNeighbourUpdateRouterEvents(t *testing.T) {
	m, bus := newTestMonitor()
	m.linkNames.Store(7, "eth0")

	const global, mac, other = "2001:db8:1::9", "02:00:00:00:00:09", "02:00:00:00:00:0a"
	for _, update := range []netlink.NeighUpdate{
		neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_REACHABLE),
		neighUpdate(unix.RTM_NEWNEIGH, global, mac, netlink.NUD_STALE),
		neighUpdate(unix.RTM_NEWNEIGH, global, other, netlink.NUD_STALE),
		neighUpdate(unix.RTM_NEWNEIGH, global, "", netlink.NUD_FAILED),
		neighUpdate(unix.RTM_NEWNEIGH, "fe80::9", other, netlink.NUD_PERMANENT),
		neighUpdate(unix.RTM_DELNEIGH, global, other, netlink.NUD_REACHABLE),
		neighUpdate(unix.RTM_NEWNEIGH, "192.0.2.9", mac, netlink.NUD_REACHABLE),
	} {
		m.handleNeighUpdate(update)
	}
	if discovered, lost := routerEvents(bus); discovered != 0 || lost != 0 {
		t.Fatalf("updates without NTF_ROUTER emitted %d discovered and %d lost router events, want none", discovered, lost)
	}

	router := neighUpdate(unix.RTM_NEWNEIGH, "fe80::9", other, netlink.NUD_REACHABLE)
	router.Flags = netlink.NTF_ROUTER
	m.handleNeighUpdate(router)
	m.handleNeighUpdate(router) // a repeat announces nothing new
	if discovered, lost := routerEvents(bus); discovered != 1 || lost != 0 {
		t.Fatalf("after NTF_ROUTER: %d discovered, %d lost; want 1 and 0", discovered, lost)
	}

	m.handleNeighUpdate(neighUpdate(unix.RTM_DELNEIGH, "fe80::9", other, netlink.NUD_REACHABLE))
	if discovered, lost := routerEvents(bus); discovered != 1 || lost != 1 {
		t.Fatalf("after delete: %d discovered, %d lost; want 1 and 1", discovered, lost)
	}
}
