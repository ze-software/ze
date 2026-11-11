// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing MPLS install.
// srFIB is the OSPF-SR producer on the mpls-fib bus (the same seam LDP and
// RSVP-TE use). It applies the shared NP/E/M truth table (internal/plugins/ospf/sr)
// to apply ingress push, transit swap and PHP/Adj-SID pop entries through the
// writer of the selected data plane (fib-kernel on the Linux kernel, fib-vpp on
// VPP); OSPF names no writer. Two distinct Source tags keep the two address
// families separable in diagnostics.
// RFC: rfc/short/rfc8665.md (§5 outgoing-label rules); rfc/short/rfc8666.md (§6)

package ospf

import (
	"log/slog"
	"net/netip"
	"sync"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/pkg/ze"
)

// mpls-fib Source tags owned by OSPF SR. Values 1 (RSVP-TE) and 2 (LDP) are
// taken; OSPF SR takes the next two. The tag is diagnostic/ownership only:
// a writer switches on Action and Op, never on Source.
const (
	mplsSourceOSPFSR   uint16 = 3
	mplsSourceOSPFv3SR uint16 = 4
)

// srFIB translates SR forwarding decisions into (mpls-fib, entry) batches it
// applies through the writer of the selected data plane. Its three sets record
// what SR WANTS installed, not what a writer accepted: an entry a writer
// refused, or one applied before any writer was running (fib-vpp opens when
// VPP connects), stays wanted, and replay re-applies it when a writer asks.
// The sets also make removal idempotent. Safe for concurrent use.
type srFIB struct {
	bus    ze.EventBus
	log    *slog.Logger
	source uint16

	// mu guards the sets and is held across each Apply, so the set update and
	// the bus batch stay ordered; no label writer calls back into srFIB.
	mu     sync.Mutex
	pushed map[string]mplsfibevents.Entry
	// pops and swaps are keyed by in-label, and an in-label is in at most one
	// of them: a writer holds one entry per in-label, so a SID label that moves
	// between swap and pop replaces the old entry, and replay must not re-apply
	// both in an undefined order.
	pops  map[uint32]mplsfibevents.Entry
	swaps map[uint32]mplsfibevents.Entry
	// refusals holds the last entry for each slot that no writer carried
	// out, with the cause, until a later apply of the same slot succeeds.
	// mu guards it. The ospf-sr-label-entries doctor check reports it
	// (sr_doctor_label.go).
	refusals mplsfibevents.Refusals
}

// newSRFIB builds an SR mpls-fib producer stamped with the given Source tag.
func newSRFIB(bus ze.EventBus, source uint16) *srFIB {
	return &srFIB{
		bus:    bus,
		log:    slog.Default(),
		source: source,
		pushed: make(map[string]mplsfibevents.Entry),
		pops:   make(map[uint32]mplsfibevents.Entry),
		swaps:  make(map[uint32]mplsfibevents.Entry),
	}
}

// emit applies e through the writer of the selected data plane and waits for
// its acknowledgement. A refusal, or no writer at all, is recorded and logged
// with the entry and its cause rather than lost: an added entry stays wanted,
// and a writer that opens later asks for it again (replay). Caller holds f.mu.
func (f *srFIB) emit(e mplsfibevents.Entry) {
	if f.bus == nil {
		// With no bus nothing was ever installed, so a removal leaves the
		// data plane as SR wants it: only an add is unprogrammable.
		if e.Action == mplsfibevents.ActionRemove {
			f.refusals.Clear(&e)
			return
		}
		f.refusals.Record(&e, errSRNoEventBus)
		f.log.Warn("ospf-sr: no event bus, so this Segment Routing label entry cannot reach a forwarding writer; Segment Routing labels are programmed only when OSPF runs inside the daemon; run `ze show doctor` to list the entries",
			"op", e.Op, "in-label", e.InLabel, "fec", e.FEC, "source", e.Source,
			"code", mplsfibevents.CodeDoctorNoWriter)
		return
	}
	if err := mplsfibevents.Apply(f.bus, []mplsfibevents.Entry{e}); err != nil {
		f.refusals.Record(&e, err)
		f.log.Warn("ospf-sr: the forwarding writer refused a Segment Routing label entry, so the data plane does not hold what SR wants for this SID; an added entry stays wanted and is applied again when a writer opens; run `ze show doctor` to see why the data plane's writer refused it",
			"action", e.Action, "op", e.Op, "fec", e.FEC, "in-label", e.InLabel,
			"out-labels", e.OutLabels, "next-hop", e.NextHop, "source", e.Source, "error", err,
			"code", mplsfibevents.CodeDoctorNoWriter)
		return
	}
	f.refusals.Clear(&e)
}

// programPush installs an ingress push (FEC -> OutLabels toward NextHop).
func (f *srFIB) programPush(fec netip.Prefix, label uint32, nh netip.Addr) {
	e := mplsfibevents.Entry{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpPush,
		FEC:       fec,
		OutLabels: []uint32{label},
		NextHop:   nh,
		Source:    f.source,
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pushed[fec.String()] = e
	f.emit(e)
}

// removePush withdraws an ingress push. It is idempotent: removing a FEC that is
// not installed is a no-op (RFC 8665 §5 PHP forwards as plain IP with no push).
func (f *srFIB) removePush(fec netip.Prefix) {
	key := fec.String()
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, wanted := f.pushed[key]; !wanted {
		return
	}
	delete(f.pushed, key)
	f.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpPush, FEC: fec, Source: f.source})
}

// programSwap installs a transit swap (InLabel -> OutLabel toward NextHop). It
// replaces a pop wanted for the same in-label.
func (f *srFIB) programSwap(inLabel, outLabel uint32, nh netip.Addr) {
	e := mplsfibevents.Entry{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpSwap,
		InLabel:   inLabel,
		OutLabels: []uint32{outLabel},
		NextHop:   nh,
		Source:    f.source,
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.pops, inLabel)
	f.swaps[inLabel] = e
	f.emit(e)
}

// removeSwap withdraws a transit swap keyed by InLabel (idempotent).
func (f *srFIB) removeSwap(inLabel uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, wanted := f.swaps[inLabel]; !wanted {
		return
	}
	delete(f.swaps, inLabel)
	f.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpSwap, InLabel: inLabel, Source: f.source})
}

// programPop installs a pop/forward entry keyed by InLabel (PHP transit and
// Adj-SID egress). It replaces a swap wanted for the same in-label.
func (f *srFIB) programPop(inLabel uint32, nh netip.Addr) {
	e := mplsfibevents.Entry{
		Action:  mplsfibevents.ActionAdd,
		Op:      mplsfibevents.OpPop,
		InLabel: inLabel,
		NextHop: nh,
		Source:  f.source,
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.swaps, inLabel)
	f.pops[inLabel] = e
	f.emit(e)
}

// removePop withdraws a pop entry keyed by InLabel (idempotent). Used to withdraw
// an Adj-SID when the adjacency drops below 2-Way (RFC 8665 §7.4.1 / RFC 8666 §8.4.1).
func (f *srFIB) removePop(inLabel uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, wanted := f.pops[inLabel]; !wanted {
		return
	}
	delete(f.pops, inLabel)
	f.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpPop, InLabel: inLabel, Source: f.source})
}

// subscribeReplay makes f answer (mpls-fib, replay-request) on its bus and
// returns the unsubscribe function, which the engine MUST call on shutdown.
// With no bus it subscribes nothing and returns a no-op.
func (f *srFIB) subscribeReplay() func() {
	if f.bus == nil {
		return func() {}
	}
	return mplsfibevents.ReplayRequest.Subscribe(f.bus, f.replay)
}

// replay answers a label writer's (mpls-fib, replay-request): it re-applies
// every entry SR wants installed, tagged with the request's token, so only the
// writer that asked consumes it. A writer opens after SPF installed the SR
// labels (VPP connects late), or after it lost its FIB (VPP restarted), and
// holds none of them until this answer.
//
// It runs on the requesting writer's goroutine, synchronously, and takes
// f.mu: the writer MUST NOT hold the lock its own (mpls-fib, entry) handler
// takes while it emits the request.
func (f *srFIB) replay(req *replay.Request) {
	if req == nil {
		return
	}
	if !replay.IsReplay(req.ReplayID) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	entries := make([]mplsfibevents.Entry, 0, len(f.pops)+len(f.swaps)+len(f.pushed))
	for _, e := range f.pops {
		entries = append(entries, e)
	}
	for _, e := range f.swaps {
		entries = append(entries, e)
	}
	for _, e := range f.pushed {
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return
	}
	if err := mplsfibevents.ApplyReplay(f.bus, req.ReplayID, entries); err != nil {
		// The writer answers one acknowledgement for the batch, so which
		// entries it refused is unknown: each is recorded with the cause.
		for i := range entries {
			f.refusals.Record(&entries[i], err)
		}
		f.log.Warn("ospf-sr: the forwarding writer that asked for the Segment Routing label entries did not install them all, so labeled traffic for those SIDs is not forwarded; run `ze show doctor` to see why",
			"entries", len(entries), "source", f.source, "error", err,
			"code", mplsfibevents.CodeDoctorNoWriter)
		return
	}
	for i := range entries {
		f.refusals.Clear(&entries[i])
	}
}

// installPrefixSID programs the ingress and transit forwarding for one reachable
// Prefix-SID toward one next-hop. action is the already-resolved forwarding action
// (the caller applies the NP/E/M truth table only at the penultimate hop, RFC 8665
// §5 / RFC 8666 §6, and swaps unconditionally at a transit hop); outLabel is the
// label computed from the NEXT-HOP router's SRGB; myLabel is this node's own incoming
// label for the SID (its SRGB[index]); explicitNull is the address family's Explicit
// NULL label.
func (f *srFIB) installPrefixSID(fec netip.Prefix, action sr.OutgoingAction, myLabel uint32, myLabelOK bool, outLabel, explicitNull uint32, nh netip.Addr) {
	// Ingress (local traffic to the prefix): push, or forward as plain IP on PHP.
	if action == sr.ActionPHP {
		f.removePush(fec)
	} else {
		label, _ := sr.OutgoingLabel(outLabel, action, explicitNull)
		f.programPush(fec, label, nh)
	}
	// Transit (labeled traffic arriving with our own SID label): swap or pop.
	if myLabelOK {
		if action == sr.ActionPHP {
			f.programPop(myLabel, nh)
		} else {
			label, _ := sr.OutgoingLabel(outLabel, action, explicitNull)
			f.programSwap(myLabel, label, nh)
		}
	}
}

// installAdjSID installs a pop/forward entry for one of this node's Adj-SIDs: the
// local SRLB label is popped and the packet forwarded to the specific adjacency,
// bypassing SPF (RFC 8665 §6.1 / RFC 8666 §7.1).
func (f *srFIB) installAdjSID(label uint32, nh netip.Addr) { f.programPop(label, nh) }

// withdrawAdjSID removes an Adj-SID pop entry.
func (f *srFIB) withdrawAdjSID(label uint32) { f.removePop(label) }
