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
	"log/slog"
	"net/netip"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/ze"
)

// mplsSourceRSVPTE tags MPLS forwarding entries emitted by RSVP-TE for
// diagnostics and ownership on the fib-kernel side.
const mplsSourceRSVPTE uint16 = 1

type busFIB struct {
	bus ze.EventBus
	log *slog.Logger
}

func newBusFIB(bus ze.EventBus, log *slog.Logger) *busFIB {
	return &busFIB{bus: bus, log: log}
}

func (b *busFIB) emit(e mplsfibevents.Entry) {
	e.Source = mplsSourceRSVPTE
	if b.bus == nil {
		b.log.Warn("rsvp-te: no event bus, cannot program MPLS entry", "op", e.Op)
		return
	}
	batch := &mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{e}}
	if _, err := mplsfibevents.EntryChange.Emit(b.bus, batch); err != nil {
		b.log.Warn("rsvp-te: mpls-fib emit failed", "op", e.Op, "error", err)
	}
}

func (b *busFIB) programPush(fec netip.Prefix, label uint32, nextHop netip.Addr) error {
	b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush, FEC: fec, OutLabels: []uint32{label}, NextHop: nextHop})
	return nil
}

func (b *busFIB) programSwap(inLabel, outLabel uint32, nextHop netip.Addr) error {
	b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: inLabel, OutLabels: []uint32{outLabel}, NextHop: nextHop})
	return nil
}

// programBackup emits a swap whose OutLabels is the facility-backup stack (bypass
// label over the swapped protected label). fib-kernel's addMPLSSwap programs the
// whole stack on the AF_MPLS route (RFC 4090 Section 3.2); the entry replaces the
// single-label swap installed for this in-label.
func (b *busFIB) programBackup(inLabel uint32, outLabels []uint32, nextHop netip.Addr) error {
	b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: inLabel, OutLabels: outLabels, NextHop: nextHop})
	return nil
}

func (b *busFIB) programPop(inLabel uint32, nextHop netip.Addr) error {
	b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPop, InLabel: inLabel, NextHop: nextHop})
	return nil
}

func (b *busFIB) removePush(fec netip.Prefix) error {
	b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpPush, FEC: fec})
	return nil
}

func (b *busFIB) removeSwap(inLabel uint32) error {
	b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpSwap, InLabel: inLabel})
	return nil
}
