// Design: docs/architecture/ldp/mpls-ldp.md -- LDP dataplane via the mpls-fib bus (AC-3/AC-4)
// Related: register.go -- runSession wires label events to the fib
// Related: ../../core/mplsfib/events.go -- the (mpls-fib, entry) payload
//
// ldpFIB translates LDP remote label bindings into (mpls-fib, entry) push
// events that fib-kernel programs into the kernel. A binding learned from a
// peer ("FEC -> label via peer") makes this LSR an ingress that imposes that
// label on traffic to FEC and forwards it to the advertising peer. fib-kernel
// stays the single owner of kernel forwarding state; LDP never touches netlink
// directly -- the same model RSVP-TE uses.
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
	removed := lib.RemoveRemote(fec, peerKey)
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
	if b, ok := lib.RemainingForFEC(fec); ok {
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
		removed = lib.RemoveAllForPeer(peerKey)
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

	// pushed tracks the FECs for which an ingress push is currently installed, so
	// Remove is idempotent: it withdraws a push only when one exists. This makes
	// the implicit-null transition correct (a FEC relabelled real -> implicit-null
	// has its stale push cleared) without emitting spurious removals for FECs that
	// never had a push. Shared across session goroutines, so guarded by mu.
	mu     sync.Mutex
	pushed map[string]bool

	// reconcileMu serializes the compound LIB-mutation + FIB-reconciliation across
	// sessions (see withReconcileLock). Distinct from mu, which only guards pushed.
	reconcileMu sync.Mutex
}

func newLDPFIB(bus ze.EventBus, log *slog.Logger) *ldpFIB {
	return &ldpFIB{bus: bus, log: log, pushed: make(map[string]bool)}
}

func (f *ldpFIB) emit(e mplsfibevents.Entry) {
	e.Source = mplsSourceLDP
	if f.bus == nil {
		f.log.Warn("ldp: no event bus, cannot program MPLS entry", "op", e.Op)
		return
	}
	batch := &mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{e}}
	if _, err := mplsfibevents.EntryChange.Emit(f.bus, batch); err != nil {
		f.log.Warn("ldp: mpls-fib emit failed", "op", e.Op, "error", err)
	}
}

// ProgramPush installs an ingress push: traffic to fec gets label imposed and
// is forwarded to nextHop (the peer that advertised the binding).
func (f *ldpFIB) ProgramPush(fec netip.Prefix, label uint32, nextHop netip.Addr) {
	// Emit under the lock so the pushed-set update and the bus event stay ordered
	// when sessions program the same FEC concurrently. Safe: the fib-kernel
	// subscriber does not call back into ldpFIB.
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pushed[fec.String()] = true
	f.emit(mplsfibevents.Entry{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpPush,
		FEC:       fec,
		OutLabels: []uint32{label},
		NextHop:   nextHop,
	})
}

// Remove withdraws the push entry for fec if one is installed. It is idempotent:
// removing a FEC that was never pushed (e.g. an implicit-null binding) is a no-op,
// so it neither leaves a stale entry on relabel nor emits a spurious withdrawal.
func (f *ldpFIB) Remove(fec netip.Prefix) {
	key := fec.String()
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.pushed[key] {
		return
	}
	delete(f.pushed, key)
	f.emit(mplsfibevents.Entry{
		Action: mplsfibevents.ActionRemove,
		Op:     mplsfibevents.OpPush,
		FEC:    fec,
	})
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

// ProgramPop installs an egress pop: traffic arriving with the local label this
// LSR advertised for fec has the label popped and is then forwarded by normal IP
// lookup. This LSR is the egress for fec (a connected prefix or its own LSR-ID),
// so it advertises a real label (ultimate-hop popping) rather than implicit-null.
// fib-kernel keys the AF_MPLS entry by InLabel; FEC rides along for diagnostics.
func (f *ldpFIB) ProgramPop(fec netip.Prefix, inLabel uint32) {
	f.emit(mplsfibevents.Entry{
		Action:  mplsfibevents.ActionAdd,
		Op:      mplsfibevents.OpPop,
		InLabel: inLabel,
		FEC:     fec,
	})
}

// RemovePop withdraws the egress pop entry keyed by inLabel.
func (f *ldpFIB) RemovePop(fec netip.Prefix, inLabel uint32) {
	f.emit(mplsfibevents.Entry{
		Action:  mplsfibevents.ActionRemove,
		Op:      mplsfibevents.OpPop,
		InLabel: inLabel,
		FEC:     fec,
	})
}
