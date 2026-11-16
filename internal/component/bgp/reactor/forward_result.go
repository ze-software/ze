// Design: docs/architecture/bgp/structural-forwarding.md -- failed source-removal preparation ends its captured session.
// Related: forward_validation.go -- settlement after forwarding locks are released.
// Related: aigp_readvertise.go -- the replay's separate dispatch-lock owner.
package reactor

import "github.com/ze-software/ze/internal/component/bgp/message"

// forwardResult keeps ordinary forwarding outcomes separate from sessions
// whose accepted source removals could not be prepared. The zero value is an
// undecided result: a producer sets err, a sent receipt or fatal before returning.
// fatal is private to one synchronous fan-out, shared across its partitions;
// nil means no failed session and allocates nothing on the normal path.
type forwardResult struct {
	err           error
	dispatched    bool
	trackDelivery bool
	sent          [4]*Session
	extra         map[*Session]struct{}
	fatal         map[*Session]error
}

func (r *forwardResult) fail(session *Session, cause error) {
	if session == nil {
		return // No captured connection remains to retire.
	}
	if r.fatal == nil {
		r.fatal = make(map[*Session]error)
	}
	if _, exists := r.fatal[session]; exists {
		return
	}
	r.fatal[session] = cause
}

func (r forwardResult) failed(session *Session) bool {
	_, failed := r.fatal[session]
	return failed
}

func (r *forwardResult) merge(part forwardResult) {
	r.dispatched = r.dispatched || part.dispatched
	r.trackDelivery = r.trackDelivery || part.trackDelivery
	if r.fatal == nil {
		r.fatal = part.fatal
	}
	for session, cause := range part.fatal {
		r.fail(session, cause)
	}
	for _, session := range part.sent {
		r.sentTo(session)
	}
	for session := range part.extra {
		r.sentTo(session)
	}
}

// sentTo records captured connection identity. Four common destinations need
// no allocation; additional receipts are bounded by this call's actual fan-out.
func (r *forwardResult) sentTo(session *Session) {
	if session == nil {
		return
	}
	r.dispatched = true
	if !r.trackDelivery {
		return
	}
	for i, sent := range r.sent {
		if sent == session {
			return
		}
		if sent == nil {
			r.sent[i] = session
			return
		}
	}
	if r.extra == nil {
		r.extra = make(map[*Session]struct{})
	}
	r.extra[session] = struct{}{}
}

func (r forwardResult) delivered() bool {
	if !r.trackDelivery {
		return r.dispatched
	}
	return r.deliveries() != 0
}

func (r forwardResult) deliveries() int {
	count := 0
	for _, session := range r.sent {
		if session != nil {
			if !r.failed(session) {
				count++
			}
		}
	}
	for session := range r.extra {
		if !r.failed(session) {
			count++
		}
	}
	return count
}

// outcome preserves partial-delivery success. A fatal-only result must not
// claim that any survivor reached a destination.
func (r forwardResult) outcome() error {
	if r.err != nil {
		return r.err
	}
	if r.delivered() {
		return nil
	}
	if len(r.fatal) != 0 {
		return errNoEstablishedPeersToForwardTo
	}
	return nil
}

// settle MUST run after the source validation and AIGP dispatch locks are
// released: the live FSM close callback reenters AIGP bookkeeping. It closes
// only captured sessions, never a replacement found through the peer. The
// notified close intentionally takes normal BGP removal rather than GR retention.
// Not safe for concurrent use with this fan-out's producers.
func (r forwardResult) settle() {
	for session := range r.fatal {
		session.teardownAutomatic(message.NotifyCeaseOutOfResources, "") //nolint:errcheck // teardownSession always returns nil after closing this captured connection.
	}
}
