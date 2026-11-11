// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE dataplane via the mpls-fib bus
// Related: engine.go -- fibProgrammer interface the engine drives
// Related: ../../core/mplsfib/events.go -- the (mpls-fib, entry) payload
//
// busFIB is the production fibProgrammer: it translates the engine's
// push/swap/pop/remove calls into (mpls-fib, entry) events that the writer of
// the selected data plane programs: fib-kernel on the Linux kernel, fib-vpp on
// VPP. The writer stays the single owner of its data plane's forwarding
// state; RSVP-TE never touches netlink or VPP directly, and names no writer:
// its registration declares NeedsDataPlane.
package rsvpte

import (
	"net/netip"
	"sync"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// mplsSourceRSVPTE tags MPLS forwarding entries emitted by RSVP-TE for
// diagnostics and ownership on the writer's side.
const mplsSourceRSVPTE uint16 = 1

// installedKey identifies one installed entry the way a writer keys it: a
// swap or pop by its incoming label, a push by its FEC and table.
type installedKey struct {
	push    bool
	inLabel uint32
	fec     netip.Prefix
	tableID uint32
}

func keyOf(e *mplsfibevents.Entry) installedKey {
	if e.Op == mplsfibevents.OpPush {
		return installedKey{push: true, fec: e.FEC, tableID: e.TableID}
	}
	return installedKey{inLabel: e.InLabel}
}

// busFIB is safe for concurrent use: the engine's signaling goroutine, the
// link-down worker and a replay request each reach it.
type busFIB struct {
	bus ze.EventBus

	// mu guards installed and refusals.
	mu sync.Mutex
	// installed holds every entry a writer accepted and RSVP-TE has not
	// removed: the forwarding its LSPs rely on. A writer that opens later
	// asks for it again (replay). An entry a writer refused is not here: the
	// engine refused its LSP, and the LSP's own retry programs it again.
	installed map[installedKey]mplsfibevents.Entry
	// refusals holds the installed entries a writer's replay request asked
	// for and did not acknowledge as installed, with the cause, until a later
	// apply of the same slot or a successful replay. mu guards it. The
	// rsvp-te-label-entries doctor check reports it (doctor_label.go).
	refusals mplsfibevents.Refusals
}

func (b *busFIB) emit(e mplsfibevents.Entry) error {
	e.Source = mplsSourceRSVPTE
	b.mu.Lock()
	defer b.mu.Unlock()
	err := mplsfibevents.Apply(b.bus, []mplsfibevents.Entry{e})
	if err != nil {
		return err
	}
	b.recordLocked(&e)
	return nil
}

// recordLocked updates the installed set after a writer accepted e. A
// removal is recorded only once accepted, so a failed withdrawal stays
// installed and is replayed rather than forgotten. Caller holds b.mu.
func (b *busFIB) recordLocked(e *mplsfibevents.Entry) {
	b.refusals.Clear(e)
	switch e.Action {
	case mplsfibevents.ActionAdd:
		if b.installed == nil {
			b.installed = make(map[installedKey]mplsfibevents.Entry)
		}
		b.installed[keyOf(e)] = *e
	case mplsfibevents.ActionRemove:
		delete(b.installed, keyOf(e))
	case mplsfibevents.ActionRemoveLabelSource:
		for key := range b.installed {
			if !key.push {
				delete(b.installed, key)
			}
		}
		b.refusals.ClearIf(func(held *mplsfibevents.Entry) bool { return held.Op != mplsfibevents.OpPush })
	case mplsfibevents.ActionUnspecified:
		// busFIB builds every entry with a named action; nothing to record.
	}
}

// replay answers a label writer's (mpls-fib, replay-request): it re-applies
// every installed entry, tagged with the request's token, so only the writer
// that asked consumes it. A writer opens after RSVP-TE programmed its LSPs
// (VPP connects late), or after it lost its FIB (VPP restarted), and holds
// none of them until this answer.
//
// It runs on the requesting writer's goroutine, synchronously, and takes
// b.mu: the writer MUST NOT hold the lock its own (mpls-fib, entry) handler
// takes while it emits the request. A refusal is returned: the caller logs it.
func (b *busFIB) replay(req *replay.Request) error {
	if req == nil {
		return nil
	}
	if !replay.IsReplay(req.ReplayID) {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.installed) == 0 {
		return nil
	}
	entries := make([]mplsfibevents.Entry, 0, len(b.installed))
	for _, e := range b.installed {
		entries = append(entries, e)
	}
	err := mplsfibevents.ApplyReplay(b.bus, req.ReplayID, entries)
	if err != nil {
		for i := range entries {
			b.refusals.Record(&entries[i], err)
		}
		return err
	}
	b.refusals.Reset()
	return nil
}

func (b *busFIB) programPush(fec netip.Prefix, labels []uint32, nextHop netip.Addr, tableID, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush, FEC: fec, OutLabels: labels, NextHop: nextHop, TableID: tableID, PathMTU: pathMTU})
}

func (b *busFIB) programSwap(inLabel, outLabel uint32, nextHop netip.Addr, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: inLabel, OutLabels: []uint32{outLabel}, NextHop: nextHop, PathMTU: pathMTU})
}

// programBackup emits a swap whose OutLabels is the facility-backup stack (bypass
// label over the swapped protected label). The writer programs the whole
// stack on the in-label's route (RFC 4090 Section 3.2); the entry replaces the
// single-label swap installed for this in-label.
func (b *busFIB) programBackup(inLabel uint32, outLabels []uint32, nextHop netip.Addr, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap, InLabel: inLabel, OutLabels: outLabels, NextHop: nextHop, PathMTU: pathMTU})
}

// programPop emits a pop. fec names the payload's network layer protocol on an
// egress pop with no next hop (RFC 3032 Section 2.2); it is the zero prefix
// on a transit pop, whose next hop the writer forwards to.
func (b *busFIB) programPop(inLabel uint32, fec netip.Prefix, nextHop netip.Addr, pathMTU uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPop, InLabel: inLabel, FEC: fec, NextHop: nextHop, PathMTU: pathMTU})
}

func (b *busFIB) removePush(fec netip.Prefix, tableID uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpPush, FEC: fec, TableID: tableID})
}

func (b *busFIB) removeSwap(inLabel uint32) error {
	return b.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpSwap, InLabel: inLabel})
}
