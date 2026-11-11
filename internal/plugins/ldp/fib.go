// Design: docs/architecture/ldp/mpls-ldp.md -- LDP dataplane via the mpls-fib bus (AC-3/AC-4)
// Related: register.go -- runSession wires label events to the fib
// Related: ../../core/mplsfib/events.go -- the (mpls-fib, entry) payload
//
// ldpFIB translates LDP label bindings into (mpls-fib, entry) events that the
// writer of the selected data plane programs: fib-kernel on the Linux kernel,
// fib-vpp on VPP. A binding learned from a peer ("FEC -> label via peer")
// makes this LSR an ingress that imposes that label on traffic to FEC and
// forwards it to the advertising peer; a local binding is an egress pop. The
// writer stays the single owner of its data plane's forwarding state; LDP
// never touches netlink or VPP directly -- the same model RSVP-TE uses. LDP
// names no writer: its registration declares NeedsDataPlane.
//
// NOTE (next-hop resolution): this phase programs the push toward the peer that
// advertised the binding, i.e. it assumes that peer is the IP next-hop for the
// FEC. That holds for directly-connected LDP neighbors (the common 2-node and
// linear cases). Resolving the binding against the IGP/FIB next-hop, and
// transit swap (which needs local label advertisement, AC-3), are follow-ups.
package ldp

import (
	"log/slog"
	"net/netip"
	"sync"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/ze"
)

// applyRemoteBinding programs kernel forwarding for a label mapping learned from a
// peer. RFC 5036 Section 3.5.7 / RFC 3032 Section 2.1: implicit-null (3) is the
// penultimate-hop-pop signal -- the advertising peer is the egress and wants no
// label imposed, so ze forwards the FEC as plain IP and must NOT push label 3.
// Explicit-null (0) is a real on-wire label and is imposed like any other.
func applyRemoteBinding(fib *ldpFIB, fec netip.Prefix, label uint32, nextHop netip.Addr, log *slog.Logger) {
	if label == ImplicitNull {
		// Forward as plain IP, and clear any push a prior real-label advertisement
		// for this FEC installed (Remove is a no-op if none was). Without this, a
		// real -> implicit-null relabel would leave the old push imposing a label.
		log.Debug("ldp: implicit-null binding, forwarding FEC as plain IP (no push)", "fec", fec, "peer", nextHop)
		fib.Remove(fec)
		return
	}
	fib.ProgramPush(fec, label, nextHop)
}

// withdrawRemoteBinding removes the LIB entry for a withdrawn FEC and reconciles
// the kernel forwarding. Returns the removed binding (nil if none). The caller
// must hold the reconcile lock (withReconcileLock).
func withdrawRemoteBinding(fib *ldpFIB, lib *LIB, fec netip.Prefix, peerKey string, log *slog.Logger) *LabelBinding {
	removed := lib.removeRemote(fec, peerKey)
	reconcileFEC(fib, lib, fec, log)
	return removed
}

// reconcileFEC programs the kernel forwarding for fec to its current best binding
// (the lowest-key peer advertising it, for a deterministic choice), or withdraws
// the push when no peer advertises it. It is the single point where a FEC's
// desired forwarding state is derived from the LIB, used by both label-mapping and
// withdraw handling so the active peer is selected consistently. The caller must
// hold the reconcile lock (withReconcileLock) so the LIB read and the FIB program
// are atomic with respect to concurrent updates for the same FEC.
func reconcileFEC(fib *ldpFIB, lib *LIB, fec netip.Prefix, log *slog.Logger) {
	if b, ok := lib.remainingForFEC(fec); ok {
		applyRemoteBinding(fib, fec, b.Label, b.NextHop, log)
		return
	}
	fib.Remove(fec)
}

// reconcilePeerDown removes all of peerKey's bindings and reconciles each affected
// FEC, taking the reconcile lock PER FEC. A peer can advertise a label mapping for
// every FEC in its RIB, and each reconcile programs the kernel synchronously, so
// holding the lock across the whole loop would stall every other session's label
// processing for the duration of a large peer's teardown. Per-FEC locking keeps
// each reconcile atomic while letting other sessions interleave between FECs.
// Returns the removed bindings (for event emission by the caller).
func reconcilePeerDown(fib *ldpFIB, lib *LIB, peerKey string, log *slog.Logger) []*LabelBinding {
	var removed []*LabelBinding
	fib.withReconcileLock(func() {
		removed = lib.removeAllForPeer(peerKey)
	})
	for _, b := range removed {
		fib.withReconcileLock(func() {
			reconcileFEC(fib, lib, b.FEC, log)
		})
	}
	return removed
}

// mplsSourceLDP tags MPLS forwarding entries emitted by LDP for diagnostics and
// ownership on the fib-kernel side (RSVP-TE uses source 1).
const mplsSourceLDP uint16 = 2

type ldpFIB struct {
	bus ze.EventBus
	log *slog.Logger

	// mu guards the two wanted sets. Shared across session goroutines.
	mu sync.Mutex
	// pushed holds the ingress push LDP wants installed for each FEC, keyed by
	// the FEC's text. It makes Remove idempotent: it withdraws a push only when
	// one is wanted, so the implicit-null transition (a FEC relabelled real ->
	// implicit-null) clears its stale push without emitting spurious removals
	// for FECs that never had one.
	pushed map[string]mplsfibevents.Entry
	// pops holds the egress pop LDP wants installed for each local label.
	pops map[uint32]mplsfibevents.Entry
	// The two sets record what LDP WANTS, not what a writer accepted: an entry
	// a writer refused, or one applied before any writer was running, stays
	// wanted, and replay re-applies it when a writer opens.

	// refusals holds the last entry for each FEC or in-label that no writer
	// carried out, with the cause, until a later apply of the same slot
	// succeeds. mu guards it. The ldp-label-entries doctor check reports it
	// (doctor_label.go).
	refusals mplsfibevents.Refusals

	// reconcileMu serializes the compound LIB-mutation + FIB-reconciliation across
	// sessions (see withReconcileLock). Distinct from mu, which only guards the sets.
	reconcileMu sync.Mutex
}

func newLDPFIB(bus ze.EventBus, log *slog.Logger) *ldpFIB {
	return &ldpFIB{bus: bus, log: log,
		pushed: make(map[string]mplsfibevents.Entry), pops: make(map[uint32]mplsfibevents.Entry)}
}

// emit applies e through the writer of the selected data plane and waits for
// its acknowledgement. A refusal is logged with the entry and its cause rather
// than lost: the entry stays wanted, and a writer that opens later asks for it
// again (replay). Caller holds f.mu.
func (f *ldpFIB) emit(e mplsfibevents.Entry) {
	e.Source = mplsSourceLDP
	if f.bus == nil {
		f.refusals.Record(&e, errNoEventBus)
		f.log.Warn("ldp: no event bus, cannot program MPLS entry", "op", e.Op,
			"code", mplsfibevents.CodeDoctorNoWriter)
		return
	}
	if err := mplsfibevents.Apply(f.bus, []mplsfibevents.Entry{e}); err != nil {
		f.refusals.Record(&e, err)
		f.log.Warn("ldp: the forwarding writer refused a label entry, so the data plane does not hold what LDP wants for this FEC; an added entry stays wanted and is applied again when a writer opens; run `ze show doctor` to see why the data plane's writer refused it",
			"action", e.Action, "op", e.Op, "fec", e.FEC, "in-label", e.InLabel,
			"out-labels", e.OutLabels, "next-hop", e.NextHop, "error", err,
			"code", mplsfibevents.CodeDoctorNoWriter)
		return
	}
	f.refusals.Clear(&e)
}

// hasPush reports whether LDP wants an ingress push installed for fec.
func (f *ldpFIB) hasPush(fec netip.Prefix) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, wanted := f.pushed[fec.String()]
	return wanted
}

// hasPop reports whether LDP wants an egress pop installed for inLabel.
func (f *ldpFIB) hasPop(inLabel uint32) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, wanted := f.pops[inLabel]
	return wanted
}

// ProgramPush installs an ingress push: traffic to fec gets label imposed and
// is forwarded to nextHop (the peer that advertised the binding).
func (f *ldpFIB) ProgramPush(fec netip.Prefix, label uint32, nextHop netip.Addr) {
	// Apply under the lock so the wanted-set update and the bus event stay
	// ordered when sessions program the same FEC concurrently. Safe: no label
	// writer calls back into ldpFIB.
	f.mu.Lock()
	defer f.mu.Unlock()
	e := mplsfibevents.Entry{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpPush,
		FEC:       fec,
		OutLabels: []uint32{label},
		NextHop:   nextHop,
		Source:    mplsSourceLDP,
	}
	f.pushed[fec.String()] = e
	f.emit(e)
}

// Remove withdraws the push entry for fec if one is installed. It is idempotent:
// removing a FEC that was never pushed (e.g. an implicit-null binding) is a no-op,
// so it neither leaves a stale entry on relabel nor emits a spurious withdrawal.
func (f *ldpFIB) Remove(fec netip.Prefix) {
	key := fec.String()
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, wanted := f.pushed[key]; !wanted {
		return
	}
	delete(f.pushed, key)
	f.emit(mplsfibevents.Entry{
		Action: mplsfibevents.ActionRemove,
		Op:     mplsfibevents.OpPush,
		FEC:    fec,
	})
}

// replay answers a label writer's (mpls-fib, replay-request): it re-applies
// every entry LDP wants installed, tagged with the request's token, so only
// the writer that asked consumes it. A writer opens after LDP's OnStarted
// programmed its egress pops (VPP connects late), or after it lost its FIB
// (VPP restarted), and holds none of them until this answer.
//
// It runs on the requesting writer's goroutine, synchronously, and takes
// f.mu: the writer MUST NOT hold the lock its own (mpls-fib, entry) handler
// takes while it emits the request.
func (f *ldpFIB) replay(req *replay.Request) {
	if req == nil {
		return
	}
	if !replay.IsReplay(req.ReplayID) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	entries := make([]mplsfibevents.Entry, 0, len(f.pops)+len(f.pushed))
	for _, e := range f.pops {
		entries = append(entries, e)
	}
	for _, e := range f.pushed {
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return
	}
	if f.bus == nil {
		for i := range entries {
			f.refusals.Record(&entries[i], errNoEventBus)
		}
		f.log.Warn("ldp: no event bus, cannot answer a label replay request", "entries", len(entries),
			"code", mplsfibevents.CodeDoctorNoWriter)
		return
	}
	if err := mplsfibevents.ApplyReplay(f.bus, req.ReplayID, entries); err != nil {
		// The writer answers one acknowledgement for the batch, so which
		// entries it refused is unknown: each is recorded with the cause.
		for i := range entries {
			f.refusals.Record(&entries[i], err)
		}
		f.log.Warn("ldp: the forwarding writer that asked for LDP's label entries did not install them all, so labeled traffic for those FECs is not forwarded; run `ze show doctor` to see why",
			"entries", len(entries), "error", err, "code", mplsfibevents.CodeDoctorNoWriter)
		return
	}
	for i := range entries {
		f.refusals.Clear(&entries[i])
	}
}

// withReconcileLock runs fn under the engine reconcile lock, which serializes the
// compound "mutate the LIB, then reconcile the FIB" operations across all sessions
// so a FEC's surviving-binding decision cannot be invalidated before it is applied.
// fib-kernel (the bus subscriber) does not call back into ldpFIB, so holding the
// lock across the emit is deadlock-free.
//
// Trade-off: because mpls-fib emits dispatch synchronously into fib-kernel's
// netlink programming, this makes LDP label installation single-threaded across
// sessions. That is an accepted choice for the control plane (correctness over
// install parallelism; netlink writes serialize anyway). Callers must keep each
// locked section to a bounded amount of work -- e.g. one FEC -- so the lock is
// never held across an unbounded synchronous loop (see reconcilePeerDown).
func (f *ldpFIB) withReconcileLock(fn func()) {
	f.reconcileMu.Lock()
	defer f.reconcileMu.Unlock()
	fn()
}

// programPop installs an egress pop: traffic arriving with the local label this
// LSR advertised for fec has the label popped and is then forwarded by normal IP
// lookup. This LSR is the egress for fec (a connected prefix or its own LSR-ID),
// so it advertises a real label (ultimate-hop popping) rather than implicit-null.
// The writer keys the entry by InLabel. FEC names the payload's network layer
// protocol, which a writer that disposes of the payload itself (fib-vpp looks
// it up in the IP table of the FEC's family) needs: RFC 3032 Section 2.2.
func (f *ldpFIB) programPop(fec netip.Prefix, inLabel uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := mplsfibevents.Entry{
		Action:  mplsfibevents.ActionAdd,
		Op:      mplsfibevents.OpPop,
		InLabel: inLabel,
		FEC:     fec,
		Source:  mplsSourceLDP,
	}
	f.pops[inLabel] = e
	f.emit(e)
}

// removePop withdraws the egress pop entry keyed by inLabel.
func (f *ldpFIB) removePop(fec netip.Prefix, inLabel uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.pops, inLabel)
	f.emit(mplsfibevents.Entry{
		Action:  mplsfibevents.ActionRemove,
		Op:      mplsfibevents.OpPop,
		InLabel: inLabel,
		FEC:     fec,
	})
}
