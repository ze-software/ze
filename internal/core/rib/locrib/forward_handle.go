// Design: plan/design-rib-rs-fastpath.md -- zero-copy forwarding for Change subscribers
// Related: change.go -- Change.Forward field carries the handle
// Related: manager.go -- InsertForward passes the handle through to dispatch

package locrib

// ForwardHandle is an opaque, reference-counted handle to the producer's
// wire buffer, attached to a Change so subscribers can forward the buffer
// without rebuilding from Change.Best.
//
// Producers (today: BGP) already hold a reference to the backing buffer
// for the duration of Insert. Subscribers that want to retain the buffer
// past the handler invocation MUST call AddRef before returning from the
// handler. The buffer stays alive until every AddRef is matched by
// Release. Subscribers that do not touch the handle pay no cost.
//
// A nil Change.Forward means the producer has no wire buffer to share --
// the subscriber MUST rebuild from Change.Best if it needs one. Kernel,
// static, and any producer that lacks a wire-shaped payload leave the
// field nil. ChangeRemove carries no handle; a ChangeUpdate synthesized
// by Remove (fallback to next-best) also carries nil today because paths
// in the PathGroup do not retain per-path buffers.
//
// Nil contract. Callers who do not have a handle MUST pass untyped nil
// (or use Insert instead of InsertForward). A typed-nil handle
// (`(*myHandle)(nil)` assigned into a ForwardHandle) produces a non-nil
// interface value whose concrete pointer is nil; subscribers doing the
// documented `if c.Forward != nil` guard would then panic on method
// dispatch. InsertForward does not normalize typed-nil to untyped-nil --
// it is the caller's responsibility.
//
// Safe for concurrent use.
type ForwardHandle interface {
	// AddRef increments the reference count. Caller MUST call Release
	// exactly once per AddRef when done with the buffer.
	//
	// Called by subscribers from inside a ChangeHandler while the RIB
	// write lock is held. Implementations MUST be cheap; anything
	// heavier (allocation, unrelated mutex, logging) serializes every
	// RIB writer behind it. A one-shot lazy copy via sync.Once is the
	// expected cost ceiling -- the first AddRef materializes an owned
	// buffer, subsequent AddRefs are pure refcount increments.
	AddRef()

	// Release decrements the reference count. The backing buffer is
	// returned to its pool when the count reaches zero. Typically called
	// off-lock from the subscriber's worker, not from the handler.
	Release()
}

// ForwardBytes is an optional capability an implementation of
// ForwardHandle may expose so subscribers can read the retained wire
// bytes. Subscribers type-assert `c.Forward.(ForwardBytes)` to access.
//
// Contract: the caller MUST have called AddRef on the handle before
// reading Bytes, and MUST NOT read Bytes after its matching Release.
// Between AddRef and Release the returned slice is safe to read from
// any goroutine. Returns nil when AddRef was never called.
type ForwardBytes interface {
	Bytes() []byte
}
