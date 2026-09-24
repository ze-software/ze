// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE dataplane via the mpls-fib bus
// Related: engine.go -- fibProgrammer interface the engine drives
// Related: ../../core/mplsfib/events.go -- the (mpls-fib, entry) payload
//
// busFIB is the production fibProgrammer: it translates the engine's
// push/swap/pop/remove calls into (mpls-fib, entry) events that fib-kernel
// programs into the kernel. fib-kernel stays the single owner of kernel
// forwarding state; RSVP-TE never touches netlink directly.
package rsvpte

import (
	"net/netip"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/ze"
)

// mplsSourceRSVPTE tags MPLS forwarding entries emitted by RSVP-TE for
// diagnostics and ownership on the fib-kernel side.
const mplsSourceRSVPTE uint16 = 1

type busFIB struct {
	bus ze.EventBus
}

func (b *busFIB) emit(e mplsfibevents.Entry) error {
	e.Source = mplsSourceRSVPTE
	return mplsfibevents.Apply(b.bus, []mplsfibevents.Entry{e})
}

func (b *busFIB) programPush(fec netip.Prefix, labels []uint32, nextHop netip.Addr, tableID, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush, FEC: fec, OutLabels: labels, NextHop: nextHop, TableID: tableID, PathMTU: pathMTU})
}

func (b *busFIB) programSwap(inLabel, outLabel uint32, nextHop netip.Addr, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: inLabel, OutLabels: []uint32{outLabel}, NextHop: nextHop, PathMTU: pathMTU})
}

// programBackup emits a swap whose OutLabels is the facility-backup stack (bypass
// label over the swapped protected label). fib-kernel's addMPLSSwap programs the
// whole stack on the AF_MPLS route (RFC 4090 Section 3.2); the entry replaces the
// single-label swap installed for this in-label.
func (b *busFIB) programBackup(inLabel uint32, outLabels []uint32, nextHop netip.Addr, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: inLabel, OutLabels: outLabels, NextHop: nextHop, PathMTU: pathMTU})
}

func (b *busFIB) programPop(inLabel uint32, nextHop netip.Addr, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPop, InLabel: inLabel, NextHop: nextHop, PathMTU: pathMTU})
}

func (b *busFIB) removePush(fec netip.Prefix, tableID uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpPush, FEC: fec, TableID: tableID})
}

func (b *busFIB) removeSwap(inLabel uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpSwap, InLabel: inLabel})
}
