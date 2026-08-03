// Design: plan/learned/805-ipsec-11-interop-eap.md -- EAP-TLS termination, peer side
// RFC: rfc/short/rfc5216.md -- EAP-TLS termination (Section 2.1.3), peer's wait
//
// RFC 5216 Section 2.1.3 puts an obligation on the PEER when it is the side that
// refuses:
//
//	"To ensure that the EAP Server receives the TLS alert message, the peer MUST
//	wait for the EAP Server to reply before terminating the conversation."
//
// rfc5216_termination_test.go asserts what the AUTHENTICATOR answers in that
// exchange (EAP-Failure). This file asserts the peer's own half: it replies
// first, parks the cause, and terminates only once the reply has arrived.
//
// The two halves are separable, and only one of them was pinned. A peer that
// reported its handshake failure INSTEAD of replying would still make the
// authenticator's EAP-Failure test pass on some runs, because the authenticator
// reaches EAP-Failure from its own stale-handshake path too -- but the alert
// would never leave the peer, and the exchange would hang until the reaper.
//
// VALIDATES: the round in which the peer's TLS engine rejects the authenticator
// returns an EAP-Response carrying the fatal alert and NO error, the cause is
// parked, and the peer reports it only after the EAP Server has replied.
// PREVENTS: PeerSession.handleTLSRequest returning PeerResult{Err} in the round
// that discovers the failure. The engine drops PeerResult.Response whenever Err
// is set, so the alert never reaches the wire, the authenticator's TLS engine
// learns nothing, and the wait Section 2.1.3 asks for is left unsatisfied.

package eap

import (
	"errors"
	"strings"
	"testing"
)

// TestRFC5216PeerRepliesBeforeItTerminates drives an exchange in which the PEER
// refuses the authenticator's certificate chain, and asserts the peer answers
// before it reports.
//
// RFC requirement: RFC5216-2.1.3-6 positive -- RFC 5216 Section 2.1.3: "To ensure
// that the EAP Server receives the TLS alert message, the peer MUST wait for the
// EAP Server to reply before terminating the conversation." The round that
// discovers the rejection returns an EAP-Response carrying the alert and no
// error, so the conversation is still open; the peer terminates on the round
// after, when the EAP Server's reply has arrived.
//
// VALIDATES: the rejecting round yields a response and no error, the cause is
// held on PeerSession.pendingErr, the authenticator answers EAP-Failure, and the
// peer then reports the chain failure by its own cause rather than the generic
// "the authenticator sent Failure".
// PREVENTS: the peer terminating in the round it discovers the failure, which
// discards the alert and leaves the authenticator waiting for a reply it never
// gets.
func TestRFC5216PeerRepliesBeforeItTerminates(t *testing.T) {
	pki := newEAPTLSPKI(t)

	// The authenticator presents a server certificate the peer cannot chain to
	// its trust anchor, so the PEER is the side that rejects.
	serverCfg := MethodConfig{
		ServerCertPEM: pki.untrustedServerCertPEM,
		ServerKeyPEM:  pki.untrustedServerKeyPEM,
		CACertPEM:     pki.trustedCAPEM, // the client certificate stays valid
	}
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	sess, err := NewSession(TypeTLS, serverCfg)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	// Run until the peer parks a failure. pendingErr is written by the branch
	// under test and by nothing else, so it names the round exactly.
	var reply, serverAnswer *Packet
	req := sess.Begin()
	for range 40 {
		pres := peer.Process(req)
		if peer.pendingErr != nil {
			if pres.Err != nil {
				t.Fatalf("the peer terminated in the round it discovered the failure (%v): the "+
					"engine drops PeerResult.Response when Err is set, so its TLS alert never "+
					"reaches the authenticator", pres.Err)
			}
			if pres.Response == nil {
				t.Fatal("the peer parked its failure and sent nothing: the authenticator is left " +
					"waiting for a reply that never comes")
			}
			reply = pres.Response
			serverAnswer = sess.Process(pres.Response)
			break
		}
		if pres.Err != nil {
			t.Fatalf("the peer failed without parking a cause: %v", pres.Err)
		}
		if pres.Done {
			t.Fatal("the peer completed against a certificate chain it cannot verify")
		}
		if pres.Response == nil {
			t.Fatal("the peer stopped answering before it rejected the authenticator")
		}
		next := sess.Process(pres.Response)
		if next == nil {
			t.Fatal("the authenticator stopped answering before the peer rejected it")
		}
		req = next
	}

	if reply == nil {
		t.Fatal("the peer never rejected the authenticator: this test needs the PEER to be the " +
			"side that refuses")
	}

	// The reply must be the alert, not a bare fragment ACK: the wait exists "to
	// ensure that the EAP Server receives the TLS alert message".
	if reply.Code != CodeResponse || reply.Type != TypeTLS {
		t.Errorf("the peer replied with code %d type %d, want an EAP-Response (%d) of "+
			"EAP-Type=EAP-TLS (%d)", reply.Code, reply.Type, CodeResponse, TypeTLS)
	}
	if bareEAPTLSResponse(reply) {
		t.Error("the peer replied with a bare fragment ACK instead of its fatal TLS alert: the " +
			"authenticator's TLS engine learns nothing and has no failure to report")
	}

	// The wait is satisfied by the EAP Server's reply, and only then does the peer
	// terminate.
	if serverAnswer == nil {
		t.Fatal("the authenticator answered the peer's alert with nothing")
	}
	if serverAnswer.Code != CodeFailure {
		t.Errorf("the authenticator answered the peer's alert with code %d, want %d (EAP-Failure)",
			serverAnswer.Code, CodeFailure)
	}
	if peer.Succeeded() {
		t.Error("the peer reports success for an authenticator it refused")
	}

	final := peer.Process(serverAnswer)
	if final.Err == nil {
		t.Fatal("the peer reported no error after the EAP Server replied: the parked cause was lost")
	}
	if final.Response != nil {
		t.Errorf("the peer answered the EAP Server's reply with %+v; the conversation terminates here",
			final.Response)
	}
	if msg := final.Err.Error(); !strings.Contains(msg, "verification failed") {
		t.Errorf("the peer reported %q, which does not name the chain failure that caused the "+
			"rejection", msg)
	}
	if peer.Succeeded() {
		t.Error("the peer reports success after terminating on a refused authenticator")
	}
}

// TestRFC5216PeerDoesNotWaitWhenItSentNothing is the boundary of the peer's wait:
// it is owed only when the peer has an alert for the EAP Server to receive.
//
// RFC requirement: RFC5216-2.1.3-6 negative -- the wait exists "to ensure that
// the EAP Server receives the TLS alert message". A peer with no trust anchor
// refuses before it starts TLS, so it has sent nothing and there is nothing for
// the EAP Server to receive: it reports in that same round and parks no cause,
// rather than waiting for a reply to a packet it never sent.
//
// VALIDATES: the refused session reports its error immediately, sends no packet,
// and leaves pendingErr unset.
// PREVENTS: the parked-cause fix turning into a hang. A peer that waited here
// would answer the authenticator until the authenticator's stale-handshake
// reaper fired, with no alert ever sent and no cause ever reported.
func TestRFC5216PeerDoesNotWaitWhenItSentNothing(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM: pki.clientCertPEM,
		KeyPEM:  pki.clientKeyPEM,
		// No CACertPEM: the peer cannot path-validate, so it refuses to start.
	})

	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	identity := peer.Process(sess.Begin())
	if identity.Response == nil {
		t.Fatalf("the peer did not answer the identity request (err=%v)", identity.Err)
	}
	start := sess.Process(identity.Response)
	if start == nil || start.Type != TypeTLS {
		t.Fatalf("the authenticator sent %+v, want the EAP-TLS Start request", start)
	}

	res := peer.Process(start)
	if res.Err == nil {
		t.Fatalf("a peer with no trust anchor started EAP-TLS anyway (response=%+v)", res.Response)
	}
	if !errors.Is(res.Err, errNoPeerTrustAnchor) {
		t.Errorf("the peer reported %v, want errNoPeerTrustAnchor", res.Err)
	}
	if res.Response != nil {
		t.Errorf("the peer sent %+v while refusing to start: there is no alert for the EAP Server "+
			"to receive, so there is nothing to wait for", res.Response)
	}
	if peer.pendingErr != nil {
		t.Errorf("the peer parked %v and waited for a reply to a packet it never sent", peer.pendingErr)
	}
}
