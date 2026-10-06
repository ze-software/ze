// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package ribevents

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
)

// RecoveryRequest names one batch of advertised routes affected by a source DOWN.
// Cut is the last received message admitted by the forwarding owner before DOWN.
// NLRIs retain native announcement framing; AddPath records identifier presence.
type RecoveryRequest struct {
	Source      netip.Addr
	Destination netip.Addr
	Family      family.Family
	NLRIs       [][]byte
	AddPath     bool
	// PrefixOnly is the text-command CIDR form of labeled unicast.
	PrefixOnly bool
	// SentAddPath selects the destination's negotiated sent identity space.
	SentAddPath bool
	Cut         uint64
}

// RecoveryRoute owns its bytes. SentNLRI names the destination's advertised
// identity. The caller MUST bind the lookup to its causal sent-delivery sequence.
// A withdrawal carries destination-framed NLRI; a replacement carries the
// surviving source's framing and its own attributes and received generation.
type RecoveryRoute struct {
	Source     netip.Addr
	NLRI       []byte
	Attributes []byte
	NextHop    []byte
	MessageID  uint64
	Withdraw   bool
	SentNLRI   []byte
}

// RecoveryOwner publishes one selecting RIB. Its owner MUST Close after event
// delivery stops. Lookup MUST be safe for concurrent use and MUST NOT call the
// forwarding engine. Selection never runs under a destination write lock.
type RecoveryOwner struct {
	Lookup func(RecoveryRequest) ([]RecoveryRoute, error)
}

var recoveryOwner atomic.Pointer[RecoveryOwner]

// PublishRecovery installs the active RIB without coupling forwarding to its
// plugin package. The caller MUST Close the returned owner after stopping it.
func PublishRecovery(lookup func(RecoveryRequest) ([]RecoveryRoute, error)) *RecoveryOwner {
	owner := &RecoveryOwner{Lookup: lookup}
	recoveryOwner.Store(owner)
	return owner
}

// Close removes only this owner's publication, never a replacement instance.
func (owner *RecoveryOwner) Close() {
	recoveryOwner.CompareAndSwap(owner, nil)
}

// RecoveryProvider returns the active in-process RIB. Nil selects the same
// producer over command IPC; it never means no surviving route exists.
func RecoveryProvider() *RecoveryOwner { return recoveryOwner.Load() }
