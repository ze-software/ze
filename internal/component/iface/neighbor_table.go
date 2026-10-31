// Design: docs/features/interfaces.md -- Bus Topics: the neighbor table BGP reads and watches, with or without an interface block
// Design: docs/architecture/update-building.md -- the third-party Link-Local BGP takes from the neighbor table
// Overview: dispatch.go -- ListNeighbors through the active backend
// Related: register.go -- installs readIPv6Neighbors and watchIPv6Neighbors as the network neighbor table
// RFC: rfc/short/rfc2545.md

package iface

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/network"
)

// readIPv6Neighbors is the neighbor reader this component registers with
// internal/core/network, so BGP reads the kernel neighbor table for RFC 2545
// Section 3's third-party Link-Local without importing this component
// (docs/architecture/core-design.md, section 19).
//
// It loads the build's default backend when none is loaded (EnsureBackend), as
// OSPF and BFD do, because a BGP-only config has no interface block to load
// one. On a platform with no OS backend the error says so.
func readIPv6Neighbors() ([]network.Neighbor, error) {
	if err := EnsureBackend(); err != nil {
		return nil, fmt.Errorf("no interface backend is loaded: %w", err)
	}
	infos, err := ListNeighbors(NeighborFamilyIPv6)
	if err != nil {
		return nil, fmt.Errorf("read the IPv6 neighbor table: %w", err)
	}
	entries := make([]network.Neighbor, 0, len(infos))
	for i := range infos {
		address, err := netip.ParseAddr(infos[i].Address)
		if err != nil {
			return nil, fmt.Errorf("neighbor table entry %q on %q: %w", infos[i].Address, infos[i].Device, err)
		}
		entries = append(entries, network.Neighbor{
			Address:     address.WithZone(""),
			LinkAddress: infos[i].MAC,
			Device:      infos[i].Device,
			Binding:     neighborBinding(infos[i].State),
		})
	}
	return entries, nil
}

// watchIPv6Neighbors is the neighbor watcher this component registers beside
// readIPv6Neighbors. It watches the table of the backend the reader reads, and
// loads the default backend the same way, so the watch runs in a BGP-only
// config with no interface block, where the interface monitor never starts.
//
// The channel it answers outlives a replaced backend. Closing a backend ends
// that backend's watch (Backend.WatchNeighbors); when a reload, the boot
// sequence's second LoadBackend or a VPP reconnect replaced it, this watch
// moves to the backend that replaced it and signals once, so the caller reads
// the new backend's table. The channel closes only with ctx, when the
// backend's own watch failed, when the backend was closed with nothing in its
// place, or when the replacing backend refuses a watch: the caller then knows
// the table is no longer watched, and its next watch names why.
//
// Each watch is one goroutine (followNeighborWatch) for the watch's life; it
// closes the answered channel last.
func watchIPv6Neighbors(ctx context.Context) (<-chan struct{}, error) {
	if err := EnsureBackend(); err != nil {
		return nil, fmt.Errorf("no interface backend is loaded: %w", err)
	}
	b, err := backendOrErr()
	if err != nil {
		return nil, fmt.Errorf("no interface backend is loaded: %w", err)
	}
	changes, err := b.WatchNeighbors(ctx)
	if err != nil {
		return nil, fmt.Errorf("watch the IPv6 neighbor table: %w", err)
	}
	out := make(chan struct{}, 1)
	go followNeighborWatch(ctx, b, changes, out)
	return out, nil
}

// followNeighborWatch forwards the signals of backend b's watch to out, and
// moves the watch to the backend that replaced b when b's watch ends. It
// closes out when it stops following, which is after the backend's channel
// closed, so no backend watch outlives it.
//
// The replacement test reads the active backend after b's channel closed.
// LoadBackend installs the new backend before it closes the old one, and
// CloseBackend clears the slot before it releases backendsMu, both under that
// lock, so GetBackend answers the replacing backend, or nil for a close with
// none, never b once b was closed by either.
func followNeighborWatch(ctx context.Context, b Backend, changes <-chan struct{}, out chan struct{}) {
	defer close(out)
	for {
		for range changes {
			signalNeighborChange(out)
		}
		if ctx.Err() != nil {
			return
		}
		next := GetBackend()
		if next == nil {
			return // closed with no backend in its place
		}
		if next == b {
			return // the backend's own watch failed: the caller retries it
		}
		nextChanges, err := next.WatchNeighbors(ctx)
		if err != nil {
			return // the caller's next watch reports why the new backend refused
		}
		b, changes = next, nextChanges
		// The replacing backend's table can differ from the one last read.
		signalNeighborChange(out)
	}
}

// signalNeighborChange leaves one pending signal on out and never blocks: the
// caller re-reads the whole table, so one pending signal covers any number of
// changes.
func signalNeighborChange(out chan struct{}) {
	select {
	case out <- struct{}{}:
	default:
	}
}

// neighborBinding classifies a backend state name (NeighborStateResolved).
func neighborBinding(state string) network.NeighborBinding {
	if NeighborStateResolved(state) {
		return network.NeighborBindingValid
	}
	return network.NeighborBindingInvalid
}

// NeighborStateResolved reports whether a NeighborInfo state name binds the
// neighbor's address to a working link-layer address. The set is open: a
// backend may report a state this component does not name, and an unknown
// state binds nothing, because only the five states below name a working
// link-layer address (Linux keeps a working neighbor in STALE most of the
// time). The netlink neighbor watch classifies its binding crossings with the
// same function, so a watch signal and the table read agree.
func NeighborStateResolved(state string) bool {
	switch state {
	case "reachable", "stale", "delay", "probe", "permanent":
		return true
	default:
		return false
	}
}
