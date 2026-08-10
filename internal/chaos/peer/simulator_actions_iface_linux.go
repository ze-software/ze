//go:build linux

// Design: docs/architecture/chaos-web-dashboard.md — netns-scoped interface fault actions
//
// Interface fault actions (spec followup-test-infra AC-6). These manipulate a
// network interface via netlink and are meant to run inside a named network
// namespace with a veth carrying the BGP session (see the integration test).
// On a plain loopback chaos run no iface is configured, so the actions are a
// graceful no-op.

package peer

import (
	"fmt"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/chaos/engine"
)

// executeIfaceLinkFlap brings the configured interface down then up (Cycles
// times), netns-scoped. Returns Disconnected=true when a flap was actually
// performed -- the session's transport is torn and must reconnect. With no
// iface param it is a graceful no-op (Disconnected=false).
func executeIfaceLinkFlap(action engine.ChaosAction, emit func(Event)) chaosResult {
	params := engine.ParseIfaceFaultParams(action.Params)
	if params.Iface == "" {
		return chaosResult{Disconnected: false}
	}
	if err := flapLink(params.Iface, params.Cycles, params.Interval); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("iface-link-flap %s: %w", params.Iface, err)})
		return chaosResult{Disconnected: false}
	}
	return chaosResult{Disconnected: true}
}

// executeIfaceAddrRemove removes then restores an address on the configured
// interface, netns-scoped. With no iface/addr param it is a graceful no-op.
func executeIfaceAddrRemove(action engine.ChaosAction, emit func(Event)) chaosResult {
	params := engine.ParseIfaceFaultParams(action.Params)
	if params.Iface == "" || params.Addr == "" {
		return chaosResult{Disconnected: false}
	}
	if err := cycleAddr(params.Iface, params.Addr, params.Interval); err != nil {
		emit(Event{Type: EventError, Err: fmt.Errorf("iface-addr-remove %s %s: %w", params.Iface, params.Addr, err)})
	}
	return chaosResult{Disconnected: false}
}

// flapLink brings the named link down then up, cycles times, pausing interval
// between each phase transition. Exported to the integration test via the
// package.
func flapLink(name string, cycles int, interval time.Duration) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("link %q: %w", name, err)
	}
	for i := range cycles {
		if err := netlink.LinkSetDown(link); err != nil {
			return fmt.Errorf("set down: %w", err)
		}
		time.Sleep(interval)
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("set up: %w", err)
		}
		if i+1 < cycles {
			time.Sleep(interval)
		}
	}
	return nil
}

// cycleAddr removes the given CIDR from the interface, waits interval, then
// restores it.
func cycleAddr(name, cidr string, interval time.Duration) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("link %q: %w", name, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("parse addr %q: %w", cidr, err)
	}
	if err := netlink.AddrDel(link, addr); err != nil {
		return fmt.Errorf("addr del: %w", err)
	}
	time.Sleep(interval)
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("addr add: %w", err)
	}
	return nil
}
