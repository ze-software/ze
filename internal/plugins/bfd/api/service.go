// Design: rfc/short/rfc5882.md -- BFD client contract
// Related: events.go -- SessionRequest, Key, StateChange
//
// Service is the interface BFD clients use to ask for and release sessions
// and to receive state-change notifications.
//
// The interface is declared here, in the public api package, so external
// plugins can depend on it without pulling in the engine runtime.
package api

// Service is the consumer contract exported by a running BFD engine.
//
// Thread safety: all methods on Service are safe for concurrent use.
// Subscribers must drain their channel; the engine never blocks on a
// slow consumer (events beyond the subscription buffer are dropped).
type Service interface {
	// EnsureSession returns a handle to a session matching req. If a
	// session with the same Key already exists, its refcount is bumped
	// and the existing handle is returned. Otherwise the engine creates
	// the session and begins the RFC 5880 slow-start timer.
	//
	// Caller MUST call ReleaseSession on the returned handle when done.
	EnsureSession(req SessionRequest) (SessionHandle, error)

	// ReleaseSession decrements the session's refcount. When the count
	// reaches zero the engine tears the session down (unless the session
	// is administratively pinned).
	//
	// Caller MUST NOT use the handle after ReleaseSession returns.
	ReleaseSession(SessionHandle) error
}

// SessionHandle is an opaque reference to a live session. Clients use it
// to subscribe to state changes and to release the session when done.
type SessionHandle interface {
	// Key returns the session identity.
	Key() Key

	// Subscribe returns a channel that receives StateChange events for
	// this session. The channel has a small buffer; if the subscriber
	// cannot keep up, older events are dropped.
	//
	// Caller MUST call Unsubscribe on the returned channel when done.
	Subscribe() <-chan StateChange

	// Unsubscribe stops delivery on a channel previously returned by
	// Subscribe. It is safe to call Unsubscribe on a channel that has
	// already been closed.
	Unsubscribe(<-chan StateChange)

	// Shutdown forces the session into AdminDown. RFC 5880 §6.8.16: an
	// administratively-disabled session ceases to transmit and discards
	// received packets except to update RemoteDiscr. Idempotent.
	Shutdown() error

	// Enable transitions the session out of AdminDown back to Down so
	// the handshake can resume. Idempotent: a no-op if the session is
	// not currently AdminDown.
	Enable() error
}
