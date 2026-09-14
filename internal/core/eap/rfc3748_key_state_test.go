// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's ticket key
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's ticket cache
// RFC: rfc/short/rfc3748.md -- the Section 7.10 key-state row
//
// EAP negotiates no key lifetime, so one party can hold key state the other has
// thrown away. The state at stake here is the EAP-TLS resumption state: the peer
// keeps the session ticket an earlier exchange issued it, and the authenticator
// keeps the ticket key that ticket was encrypted under.
//
// VALIDATES: a peer offering a ticket an authenticator can no longer decrypt
// still authenticates, through a full handshake, and the same offer against the
// authenticator that kept its key state resumes.
// PREVENTS: an authenticator that restarts, or rotates its ticket key past the
// ticket, refusing every peer that still holds one.

package eap

import (
	"testing"
	"time"
)

// TestRFC3748AnExchangeOutlivesDiscardedKeyState stocks a ticket on the peer,
// then runs the same peer store against an authenticator that kept its key state
// and against one that discarded it.
func TestRFC3748AnExchangeOutlivesDiscardedKeyState(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	initial, initialPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, initial, "the initial authentication")
	if initialPeer.Resumed() {
		t.Fatal("the initial authentication resumed a session that did not exist")
	}
	if cachedTicket(t, peerStore, "the initial authentication") == nil {
		t.Fatal("the peer stored no ticket, so it holds no key state for an authenticator to discard")
	}

	// RFC requirement: RFC3748-7.10-7 negative -- the peer's key state is live and
	// an authenticator that still holds its own half redeems it. The same peer
	// store, offered to the authenticator that issued the ticket, resumes: both
	// ends report a resumed session. So the full handshake below is the discarded
	// state and not a peer that never resumes.
	kept, keptPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, kept, "the exchange against an authenticator that kept its key state")
	if !keptPeer.Resumed() {
		t.Fatal("the peer ran a full handshake against the authenticator that issued its ticket")
	}
	if !kept.sess.Resumed() {
		t.Fatal("the authenticator ran a full handshake with a ticket it had issued itself")
	}

	// RFC requirement: RFC3748-7.10-7 positive -- RFC 3748 Section 7.10: "Since
	// EAP does not provide for explicit key lifetime negotiation, EAP peers,
	// authenticators, and authentication servers MUST be prepared for situations
	// in which one of the parties discards the key state, which remains valid on
	// another party." The authenticator here discards its key state, which is what
	// a restarted one has: its session-ticket keys are gone, so the ticket the
	// peer still holds decrypts under none of them. The peer offers it anyway, and
	// the exchange is not refused: neither end resumes, a full handshake runs, and
	// both authenticate and agree on a fresh MSK.
	discarded := NewResumption(time.Now, true)
	after, afterPeer := driveResumingExchange(t, pki, discarded, peerStore)
	requireCompleted(t, after, "the exchange against an authenticator that discarded its key state")
	if afterPeer.Resumed() {
		t.Fatal("the peer resumed against an authenticator holding none of the key state")
	}
	if after.sess.Resumed() {
		t.Fatal("the authenticator resumed a session it holds no ticket key for")
	}
	if after.peerMSK == kept.peerMSK {
		t.Fatal("the exchange after the discard derived the key state's own MSK")
	}
}
