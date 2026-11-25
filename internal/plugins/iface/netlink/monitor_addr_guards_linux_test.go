//go:build linux

// Design: docs/features/interfaces.md -- Netlink monitor for interfaces
// Related: monitor_linux.go -- handleAddrUpdate input guards.

package ifacenetlink

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
)

// Goal: an address update handleAddrUpdate cannot use as an interface address
// is dropped before anything is announced, retained or looked up, while a
// usable update on the same monitor is still announced.
// Method: drive handleAddrUpdate with one update per guard, on a monitor whose
// link lookup counts its calls, then with gatewayAddr as the accepted control.
// Each refused case is a new address (NewAddr true), so a guard that let it
// through would emit addr-added and retain it.
// Expected: no event, no retained address and no link lookup for each refused
// update; one addr-added and one retained address for the control.
func TestHandleAddrUpdate_RefusesUnusableAddresses(t *testing.T) {
	newAddr := func(ip net.IP, mask net.IPMask) netlink.AddrUpdate {
		return netlink.AddrUpdate{
			LinkIndex:   9,
			LinkAddress: net.IPNet{IP: ip, Mask: mask},
			Flags:       unix.IFA_F_PERMANENT,
			NewAddr:     true,
		}
	}
	cases := []struct {
		name   string
		update netlink.AddrUpdate
	}{
		// netip.AddrFromSlice refuses a slice that is neither 4 nor 16 bytes.
		// The masks are the ones the bindings' parseAddr builds,
		// CIDRMask(prefixlen, 8*len(IFA_ADDRESS)). Only the empty address
		// reaches the mask guard below with a matching length, so it is the
		// case that proves this guard on its own.
		{"no address bytes", newAddr(net.IP{}, net.CIDRMask(0, 0))},
		{"three address bytes", newAddr(net.IP{198, 51, 100}, net.CIDRMask(24, 24))},
		// IsUnspecified: 0.0.0.0 and :: name no interface address.
		{"unspecified IPv4", newAddr(net.IPv4zero.To4(), net.CIDRMask(8, 32))},
		{"unspecified IPv6", newAddr(net.IPv6unspecified, net.CIDRMask(64, 128))},
		// IsMulticast: a group is joined, not assigned.
		{"multicast IPv4", newAddr(net.ParseIP("224.0.0.5").To4(), net.CIDRMask(32, 32))},
		{"multicast IPv6", newAddr(net.ParseIP("ff02::1"), net.CIDRMask(128, 128))},
		// The mask must be as long as the address after Unmap.
		{"IPv4 address with an IPv6 mask", newAddr(net.ParseIP("198.51.100.7").To4(), net.CIDRMask(24, 128))},
		{"IPv4-mapped address with an IPv6 mask", newAddr(net.ParseIP("198.51.100.7"), net.CIDRMask(120, 128))},
		{"non-contiguous mask", newAddr(net.ParseIP("198.51.100.7").To4(), net.IPMask{255, 0, 255, 0})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, bus := newTestMonitor()
			lookups := 0
			m.linkByIndex = func(index int) (netlink.Link, error) {
				lookups++
				link := &netlink.Dummy{}
				link.Name, link.Index, link.MTU = "zeguard0", index, 1500
				return link, nil
			}

			m.handleAddrUpdate(tc.update)

			if events := bus.snapshot(); len(events) != 0 {
				t.Errorf("refused update emitted %+v, want nothing", events)
			}
			if len(m.addresses) != 0 {
				t.Errorf("refused update retained %+v, want nothing", m.addresses)
			}
			if lookups != 0 {
				t.Errorf("refused update looked its link up %d times, want 0", lookups)
			}

			// The same monitor still announces a usable address.
			m.handleAddrUpdate(gatewayAddr(9))
			events := bus.snapshot()
			if len(events) != 1 || events[0].EventType != ifaceevents.EventAddrAdded {
				t.Fatalf("control update emitted %+v, want one %s", events, ifaceevents.EventAddrAdded)
			}
			if len(m.addresses) != 1 {
				t.Errorf("control update retained %d addresses, want 1", len(m.addresses))
			}
		})
	}
}
