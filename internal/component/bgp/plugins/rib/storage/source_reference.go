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
	refs atomic.Int64
	wake func()
}

func NewSourceReference(wake func()) *SourceReference {
	r := &SourceReference{wake: wake}
	r.refs.Store(1)
	return r
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
