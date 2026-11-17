// Design: docs/architecture/plugin/rib-storage-design.md -- received source leases.
// Related: routeentry.go -- candidates and owned clones share this reference.
package storage

import "sync/atomic"

// SourceReference counts local owners of one engine-authorized source lease.
// Nil denotes untrusted provenance without a lease. The callback MUST only wake
// its owner: Release runs under RIB and pool locks and MUST NOT perform I/O.
// A newly constructed reference owns one count. Raw value copies borrow it;
// RouteEntry.AddRef and Clone acquire independent ownership.
type SourceReference struct {
	peerOwner uint64
	refs      atomic.Int64
	wake      func()
}

// NewSourceReference captures the original Peer owner after successful lease
// acquisition. The owner MUST supply that original fact, never a later lookup.
func NewSourceReference(peerOwner uint64, wake func()) *SourceReference {
	r := &SourceReference{peerOwner: peerOwner, wake: wake}
	r.refs.Store(1)
	return r
}

// PeerOwner is immutable provenance, not permission to send. A nil reference
// has no acquired lease and returns the explicitly unrecorded owner zero.
func (r *SourceReference) PeerOwner() uint64 {
	if r == nil {
		return 0
	}
	return r.peerOwner
}

// SourcePeerOwner reads the original Peer fact shared by owned route clones.
// A route without a lease reference returns explicitly unrecorded zero.
// The receiver MUST be non-nil.
func (e *RouteEntry) SourcePeerOwner() uint64 {
	return e.source.PeerOwner()
}

// AddRef also lets the owning lease registry re-use a pending zero reference
// before its serialized engine release starts. Other callers hold a live owner.
func (r *SourceReference) AddRef() {
	if r != nil {
		r.refs.Add(1)
	}
}

func (r *SourceReference) Release() {
	if r == nil {
		return
	}
	remaining := r.refs.Add(-1)
	if remaining < 0 {
		panic("BUG: source reference released without ownership")
	}
	if remaining == 0 {
		if r.wake != nil {
			r.wake()
		}
	}
}

func (r *SourceReference) References() int64 {
	if r == nil {
		return 0
	}
	return r.refs.Load()
}

// SetSourceReference replaces this entry's owned reference. The input is
// borrowed; the entry takes its own count before releasing the previous one.
func (e *RouteEntry) SetSourceReference(source *SourceReference) {
	if e.source == source {
		return
	}
	source.AddRef()
	e.source.Release()
	e.source = source
}
