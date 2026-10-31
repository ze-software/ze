// Design: docs/architecture/bgp/interface-event-reactions.md -- BGP reactions to interface address changes
// RFC: rfc/short/rfc2545.md — the Section 3 connected subnets follow the address table
// Overview: reactor.go — Reactor struct and lifecycle
// Related: reactor_iface.go — handleAddressAdded, handleAddressRemoved and refreshPeerLinkScopes, what a change runs
// Related: reactor_table_watch.go — superviseTableWatch, which keeps the watch open

package reactor

import (
	"context"
	"errors"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/network"
)

// watchAddressTable is the watch addressWatchWorker starts. It is
// network.WatchAddresses; a test replaces it and restores it in t.Cleanup,
// because the watcher registry is a one-shot slot filled by the netlink
// backend's init().
var watchAddressTable = network.WatchAddresses

// readAddressTable is the address table read a watch signal repeats. It is
// network.InterfacePrefixes, the read refreshPeerLinkScopes makes too; a test
// replaces it and restores it in t.Cleanup.
var readAddressTable = network.InterfacePrefixes

// waitAddressWatch is the wait between two address watch starts. It is
// waitTableWatchRetry; a test replaces it to skip the delay, and restores it
// in t.Cleanup.
var waitAddressWatch = waitTableWatchRetry

// errAddressWatchEnded is the reason of a watch that closed its channel while
// the reactor ran: its kernel subscription failed. The worker starts a new one
// after a delay.
var errAddressWatchEnded = errors.New("the interface address watch ended; ze starts it again")

// startAddressWatch opens the address watch and reads the table it watches,
// then hands both to addressWatchWorker on the reactor's wait group. The watch
// opens BEFORE the read, so an address that moves after the read is a signal
// the worker sees, and both happen before the listeners start, so an address
// removed while they start still stops its listener. MUST be called after
// r.ctx is set and before the listeners start, from StartWithContext.
//
// The watch needs no event bus and no interface config: a BGP-only config has
// no interface monitor, and the reactor still owes a listener to an address
// added after start and RFC 2545 Section 3's condition against the subnets the
// host holds now.
func (r *Reactor) startAddressWatch() {
	ctx := r.ctx
	changes, err := watchAddressTable(ctx)
	table := &addressSync{r: r}
	if err == nil {
		table.apply()
	}
	r.wg.Go(func() { r.addressWatchWorker(ctx, table, changes, err) })
}

// addressWatchWorker is the long-lived worker that applies each change of the
// kernel address table: a listener for a peer's local address that appeared,
// the end of the listener for one that went away, and RFC 2545 Section 3's
// condition re-settled for every live peer. A signal names no address, so the
// worker reads the table whole and compares it with the one it last applied
// (addressSync). superviseTableWatch keeps the watch open and starts it again
// after a failure; changes and err are the first start's answer.
func (r *Reactor) addressWatchWorker(ctx context.Context, table *addressSync, changes <-chan struct{}, err error) {
	status := addressWatchStatusUnknown
	r.superviseTableWatch(ctx, tableWatch{
		start: watchAddressTable,
		wait:  waitAddressWatch,
		ended: errAddressWatchEnded,
		running: func() {
			if status != addressWatchStatusRunning {
				reactorLogger().Info("watching the kernel interface address table for BGP listeners and RFC 2545 connected subnets")
			}
			status = addressWatchStatusRunning
			// The table may have moved while no watch ran.
			table.apply()
		},
		changed: table.apply,
		stopped: func(err error) {
			if status != addressWatchStatusStopped {
				reactorLogger().Warn("the kernel interface address table is not watched: an address added after start gets no BGP listener, and RFC 2545 next hops are judged against the subnets read at session establishment and reload",
					"error", err, "fix", addressWatchFix(err))
			}
			status = addressWatchStatusStopped
		},
	}, changes, err)
}

// addressWatchStatus is what addressWatchWorker last logged, so a retry that
// fails the same way logs nothing new. The zero value is unknown: nothing was
// logged yet.
type addressWatchStatus uint8

const (
	addressWatchStatusUnknown addressWatchStatus = iota
	addressWatchStatusRunning
	addressWatchStatusStopped
)

// addressWatchFix is the operator's next step for an address table that is
// not watched because of err.
func addressWatchFix(err error) string {
	if errors.Is(err, network.ErrAddressWatchNotRegistered) {
		return "run ze on Linux, where the netlink interface backend watches the address table; on another platform restart the BGP sessions after an address change"
	}
	return "none if the watch starts again, which ze logs; if this repeats, look for the netlink error ze logged just before this line, and for kernel messages with: dmesg | grep -i netlink"
}

// addressSync holds the address table addressWatchWorker last applied. Only
// that worker reads or writes it once startAddressWatch handed it over, so it
// needs no lock.
type addressSync struct {
	r *Reactor
	// applied is every local address the worker has acted on: an address it
	// started a listener for, or one no peer needed. An address whose
	// listener could not bind YET is left out, so the next read tries it
	// again (handleAddressAdded). Bounded by the host's address table.
	applied map[netip.Addr]struct{}
	// prefixes is the table as last read, host bits and lengths kept, so a
	// change of prefix length alone re-settles the connected subnets.
	prefixes []netip.Prefix
}

// apply reads the table and acts on what changed since the last read: each
// address that appeared, each one that went away, then one re-settling of
// every live peer's link scope when the table differs at all.
//
// The first read only records the table, with no action: startAddressWatch
// makes it before the listeners start, and they are built against it.
func (s *addressSync) apply() {
	prefixes, err := readAddressTable()
	if err != nil {
		reactorLogger().Error("read the interface address table", "error", err,
			"fix", "none: ze reads it again on the next address change")
		return
	}
	current := addressSet(prefixes)
	if s.applied == nil {
		s.prefixes = prefixes
		s.applied = current
		return
	}
	for addr := range s.applied {
		if _, present := current[addr]; present {
			continue
		}
		delete(s.applied, addr)
		s.r.handleAddressRemoved(addr)
	}
	// RFC 2545 Section 3: an address added or removed changes which subnets
	// this host is attached to, for every peer and not only the ones bound to
	// it, so the condition is re-settled before a listener accepts a session.
	if !slices.Equal(s.prefixes, prefixes) {
		s.prefixes = prefixes
		s.r.refreshPeerLinkScopes()
	}
	for addr := range current {
		if _, known := s.applied[addr]; known {
			continue
		}
		if s.r.handleAddressAdded(addr) {
			s.applied[addr] = struct{}{}
		}
	}
}

// addressSet answers the addresses of prefixes, host bits kept.
func addressSet(prefixes []netip.Prefix) map[netip.Addr]struct{} {
	set := make(map[netip.Addr]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		set[prefix.Addr().Unmap()] = struct{}{}
	}
	return set
}
