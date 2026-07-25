// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing MPLS install.
// srFIB is the OSPF-SR producer on the mpls-fib bus (the same seam LDP and
// RSVP-TE use). It applies the shared NP/E/M truth table (internal/plugins/ospf/sr)
// to emit ingress push, transit swap and PHP/Adj-SID pop entries; fib-kernel stays
// the single netlink owner. Two distinct Source tags keep the two address families
// separable in diagnostics.
// RFC: rfc/short/rfc8665.md (§5 outgoing-label rules); rfc/short/rfc8666.md (§6)

package ospf

import (
	"log/slog"
	"net/netip"
	"sync"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/pkg/ze"
)

// mpls-fib Source tags owned by OSPF SR. Values 1 (RSVP-TE) and 2 (LDP) are
// taken; OSPF SR takes the next two. The tag is diagnostic/ownership only:
// fib-kernel switches on Action and Op, never on Source.
const (
	mplsSourceOSPFSR   uint16 = 3
	mplsSourceOSPFv3SR uint16 = 4
)

// srFIB translates SR forwarding decisions into (mpls-fib, entry) events. It
// tracks installed keys so removal is idempotent, mirroring the LDP pushed-set.
type srFIB struct {
	bus    ze.EventBus
	log    *slog.Logger
	source uint16

	mu     sync.Mutex
	pushed map[string]bool
	pops   map[uint32]bool
	swaps  map[uint32]bool
}

// newSRFIB builds an SR mpls-fib producer stamped with the given Source tag.
func newSRFIB(bus ze.EventBus, source uint16) *srFIB {
	return &srFIB{
		bus:    bus,
		log:    slog.Default(),
		source: source,
		pushed: make(map[string]bool),
		pops:   make(map[uint32]bool),
		swaps:  make(map[uint32]bool),
	}
}

func (f *srFIB) emit(e mplsfibevents.Entry) {
	e.Source = f.source
	if f.bus == nil {
		return
	}
	batch := &mplsfibevents.EntryBatch{Entries: []mplsfibevents.Entry{e}}
	if _, err := mplsfibevents.EntryChange.Emit(f.bus, batch); err != nil {
		f.log.Warn("ospf-sr: mpls-fib emit failed", "op", e.Op, "error", err)
	}
}

// programPush installs an ingress push (FEC -> OutLabels toward NextHop).
func (f *srFIB) programPush(fec netip.Prefix, label uint32, nh netip.Addr) {
	f.mu.Lock()
	f.pushed[fec.String()] = true
	f.mu.Unlock()
	f.emit(mplsfibevents.Entry{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpPush,
		FEC:       fec,
		OutLabels: []uint32{label},
		NextHop:   nh,
	})
}

// removePush withdraws an ingress push. It is idempotent: removing a FEC that is
// not installed is a no-op (RFC 8665 §5 PHP forwards as plain IP with no push).
func (f *srFIB) removePush(fec netip.Prefix) {
	key := fec.String()
	f.mu.Lock()
	if !f.pushed[key] {
		f.mu.Unlock()
		return
	}
	delete(f.pushed, key)
	f.mu.Unlock()
	f.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpPush, FEC: fec})
}

// programSwap installs a transit swap (InLabel -> OutLabel toward NextHop).
func (f *srFIB) programSwap(inLabel, outLabel uint32, nh netip.Addr) {
	f.mu.Lock()
	f.swaps[inLabel] = true
	f.mu.Unlock()
	f.emit(mplsfibevents.Entry{
		Action:    mplsfibevents.ActionAdd,
		Op:        mplsfibevents.OpSwap,
		InLabel:   inLabel,
		OutLabels: []uint32{outLabel},
		NextHop:   nh,
	})
}

// removeSwap withdraws a transit swap keyed by InLabel (idempotent).
func (f *srFIB) removeSwap(inLabel uint32) {
	f.mu.Lock()
	if !f.swaps[inLabel] {
		f.mu.Unlock()
		return
	}
	delete(f.swaps, inLabel)
	f.mu.Unlock()
	f.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpSwap, InLabel: inLabel})
}

// programPop installs a pop/forward entry keyed by InLabel (PHP transit and
// Adj-SID egress).
func (f *srFIB) programPop(inLabel uint32, nh netip.Addr) {
	f.mu.Lock()
	f.pops[inLabel] = true
	f.mu.Unlock()
	f.emit(mplsfibevents.Entry{
		Action:  mplsfibevents.ActionAdd,
		Op:      mplsfibevents.OpPop,
		InLabel: inLabel,
		NextHop: nh,
	})
}

// removePop withdraws a pop entry keyed by InLabel (idempotent). Used to withdraw
// an Adj-SID when the adjacency drops below 2-Way (RFC 8665 §7.4.1 / RFC 8666 §8.4.1).
func (f *srFIB) removePop(inLabel uint32) {
	f.mu.Lock()
	if !f.pops[inLabel] {
		f.mu.Unlock()
		return
	}
	delete(f.pops, inLabel)
	f.mu.Unlock()
	f.emit(mplsfibevents.Entry{Action: mplsfibevents.ActionRemove, Op: mplsfibevents.OpPop, InLabel: inLabel})
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
