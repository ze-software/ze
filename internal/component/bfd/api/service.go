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

	// Snapshot returns a copy of every live session's observability
	// state. The returned slice is a newly allocated copy; callers may
	// mutate it freely without affecting engine state. Safe for
	// concurrent use.
	Snapshot() []SessionState

	// SessionDetail returns the same view as Snapshot but for a single
	// session matched by peer address (case-insensitive compare against
	// Key.Peer.String()). The second return is false if no session with
	// that peer exists. Safe for concurrent use.
	SessionDetail(peer string) (SessionState, bool)

	// Profiles returns a copy of every resolved (post-default) BFD
	// profile currently configured on the plugin. Order is sorted by
	// Name so operators see stable output across calls.
	Profiles() []ProfileState
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
	// The FIRST value is a SNAPSHOT of the state the session already held,
	// carrying StateChange.Initial. It exists because EnsureSession on an
	// existing key only bumps a refcount: without it a client that joins a
	// session another client already brought Up waits for a transition that
	// never comes, and can only guess at the state meanwhile. A key with no
	// session yields no snapshot, so a caller must not wait for one.
	//
	// A caller whose reaction to a state DIFFERS from its reaction to being IN
	// that state MUST branch on Initial. A session that has not come up yet
	// sits in Down (RFC 5880 Section 6.8.1), and RFC 5882 Section 4.2 asks a
	// client to react to a path that FAILED: such a client that reads the
	// snapshot as a failure tears down the adjacency it just opened. The BGP
	// and OSPF subscribers are both of that kind and both branch.
	//
	// A caller that only asks "what is the state now" needs no branch, and the
	// static-route next-hop tracker is the worked example: it compares Up
	// against what it already believed and programs on a difference, so a
	// snapshot is simply the earliest correct answer it can get. Read the
	// exception as the rule's own boundary rather than as an oversight.
	//
	// The snapshot and the registration are one atomic step, so no transition
	// can be lost between them.
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
