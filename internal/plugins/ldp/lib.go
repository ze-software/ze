// Design: docs/architecture/ldp/mpls-ldp.md -- LDP Label Information Base
// Related: wire.go -- label/FEC types
// Related: discovery.go -- adjacency keys used for peer identification
//
// RFC 5036 Section 2.6: The LIB stores FEC-to-label bindings.
// Each entry maps (FEC, peer) -> label. Downstream unsolicited mode:
// the peer sends Label Mapping without being asked.
package ldp

import (
	"net/netip"
	"sync"
)

// LabelBinding is a single FEC-to-label binding from a peer.
type LabelBinding struct {
	FEC      netip.Prefix
	Label    uint32
	PeerKey  string
	PeerAddr netip.Addr
	// NextHop is the resolved data-plane next hop for this binding (from the
	// peer's Address message, falling back to PeerAddr). Stored at learn time so a
	// later reconcile can program the binding without the originating session's
	// address list in scope.
	NextHop netip.Addr
}

// localBinding is a FEC-to-label binding this LSR originates. In downstream
// unsolicited mode the same local label is advertised to every peer (per-platform
// label space, RFC 5036 Section 2.3), so a local binding is keyed by FEC only.
type localBinding struct {
	FEC   netip.Prefix
	Label uint32
}

// LIB is the Label Information Base. Thread-safe.
type LIB struct {
	mu         sync.RWMutex
	bindings   map[string]map[string]*LabelBinding // FEC prefix string -> peer key -> binding
	local      map[string]*localBinding            // FEC prefix string -> local binding
	usedLabels map[uint32]bool                     // local labels currently allocated
	nextLabel  uint32
}

// newLIB creates an empty Label Information Base.
// Local labels start at 16 (0-15 are reserved per RFC 3032).
func newLIB() *LIB {
	return &LIB{
		bindings:   make(map[string]map[string]*LabelBinding),
		local:      make(map[string]*localBinding),
		usedLabels: make(map[uint32]bool),
		nextLabel:  16,
	}
}

// allocateLabelLocked returns the next free local label, skipping any already in
// use so the wrap from MaxLabel back to 16 cannot hand out a duplicate. Caller
// holds l.mu. Returns nextLabel unchanged only if the entire 20-bit space is
// exhausted (not reachable for the handful of local FECs an LSR originates).
func (l *LIB) allocateLabelLocked() uint32 {
	for attempts := 0; attempts <= MaxLabel-16; attempts++ {
		label := l.nextLabel
		l.nextLabel++
		if l.nextLabel > MaxLabel {
			l.nextLabel = 16
		}
		if !l.usedLabels[label] {
			l.usedLabels[label] = true
			return label
		}
	}
	return l.nextLabel
}

// AllocateLabel returns the next available local label.
func (l *LIB) AllocateLabel() uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.allocateLabelLocked()
}

// EnsureLocal returns the local binding for fec, allocating a local label on
// first use. Idempotent: repeated calls for the same FEC return the same label,
// so a FEC keeps one stable local label across all sessions and reloads.
func (l *LIB) EnsureLocal(fec netip.Prefix) localBinding {
	key := fec.String()
	l.mu.Lock()
	defer l.mu.Unlock()
	if b, ok := l.local[key]; ok {
		return *b
	}
	b := &localBinding{FEC: fec, Label: l.allocateLabelLocked()}
	l.local[key] = b
	return *b
}

// localBindings returns a snapshot of all local bindings.
func (l *LIB) localBindings() []localBinding {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]localBinding, 0, len(l.local))
	for _, b := range l.local {
		out = append(out, *b)
	}
	return out
}

// AddRemote stores a label binding received from a peer. nextHop is the resolved
// data-plane next hop for the binding (see LabelBinding.NextHop).
func (l *LIB) AddRemote(fec netip.Prefix, label uint32, peerKey string, peerAddr, nextHop netip.Addr) {
	key := fec.String()
	l.mu.Lock()
	defer l.mu.Unlock()

	peers, ok := l.bindings[key]
	if !ok {
		peers = make(map[string]*LabelBinding)
		l.bindings[key] = peers
	}
	peers[peerKey] = &LabelBinding{
		FEC:      fec,
		Label:    label,
		PeerKey:  peerKey,
		PeerAddr: peerAddr,
		NextHop:  nextHop,
	}
}

// removeRemote removes a label binding from a peer.
// Returns the removed binding, or nil if not found.
func (l *LIB) removeRemote(fec netip.Prefix, peerKey string) *LabelBinding {
	key := fec.String()
	l.mu.Lock()
	defer l.mu.Unlock()

	peers, ok := l.bindings[key]
	if !ok {
		return nil
	}
	binding := peers[peerKey]
	delete(peers, peerKey)
	if len(peers) == 0 {
		delete(l.bindings, key)
	}
	return binding
}

// removeAllForPeer removes all bindings from a specific peer.
// Returns the removed bindings.
func (l *LIB) removeAllForPeer(peerKey string) []*LabelBinding {
	l.mu.Lock()
	defer l.mu.Unlock()

	var removed []*LabelBinding
	for fecKey, peers := range l.bindings {
		if b, ok := peers[peerKey]; ok {
			removed = append(removed, b)
			delete(peers, peerKey)
		}
		if len(peers) == 0 {
			delete(l.bindings, fecKey)
		}
	}
	return removed
}

// LookupRemote returns the remote binding for a FEC from a specific peer.
func (l *LIB) LookupRemote(fec netip.Prefix, peerKey string) (*LabelBinding, bool) {
	key := fec.String()
	l.mu.RLock()
	defer l.mu.RUnlock()

	peers, ok := l.bindings[key]
	if !ok {
		return nil, false
	}
	b, ok := peers[peerKey]
	if !ok {
		return nil, false
	}
	cp := *b
	return &cp, true
}

// RemainingForFEC returns a surviving remote binding for fec, choosing the lowest
// peer key for a deterministic, stable result, or false if no peer advertises it.
// Used to re-point a FEC's push to another peer when one withdraws, rather than
// dropping forwarding for a FEC that is still reachable.
func (l *LIB) RemainingForFEC(fec netip.Prefix) (LabelBinding, bool) {
	key := fec.String()
	l.mu.RLock()
	defer l.mu.RUnlock()

	peers, ok := l.bindings[key]
	if !ok || len(peers) == 0 {
		return LabelBinding{}, false
	}
	var best *LabelBinding
	var bestKey string
	for pk, b := range peers {
		if best == nil || pk < bestKey {
			best, bestKey = b, pk
		}
	}
	return *best, true
}

// allBindings returns a snapshot of all remote bindings.
func (l *LIB) allBindings() []LabelBinding {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var out []LabelBinding
	for _, peers := range l.bindings {
		for _, b := range peers {
			out = append(out, *b)
		}
	}
	return out
}

// Len returns the total number of bindings.
func (l *LIB) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	count := 0
	for _, peers := range l.bindings {
		count += len(peers)
	}
	return count
}
