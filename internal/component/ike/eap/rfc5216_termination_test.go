// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS termination, server side
// RFC: rfc/short/rfc5216.md -- EAP-TLS termination (Section 2.1.3)
//
// RFC 5216 Section 2.1.3 ends a failed EAP-TLS conversation in two directions,
// and the EAP SERVER owes a MUST in each of them.
//
// The peer rejects the server: "To ensure that the EAP Server receives the TLS
// alert message, the peer MUST wait for the EAP Server to reply before
// terminating the conversation.  The EAP Server MUST reply with an EAP-Failure
// packet since server authentication failure is a terminal condition."
//
// The server rejects the peer: "To ensure that the peer receives the TLS alert
// message, the EAP server MUST wait for the peer to reply with an EAP-Response
// packet", and on a reply carrying no data "the EAP-Server MUST send an
// EAP-Failure packet and terminate the conversation."
//
// eap_tls_alert_flight_test.go pins the SERVER-rejects-peer flight in detail.
// This file pins the PEER-rejects-server direction, which no test reached: the
// exchange ended with the peer reporting its own error and nobody asserted what
// the authenticator answered.
//
// VALIDATES: a peer that refuses the authenticator's certificate chain sends its
// fatal TLS alert as an EAP-Response, and the authenticator answers EAP-Failure
// in the round that receives it.
// PREVENTS: the authenticator treating a peer alert as "the engine produced
// nothing this round" and answering a bare fragment ACK until the
// stale-handshake reaper fires, which is what the RFC5216-2.1.3-1 {gap}
// annotation described before tlsMethod.Process read transport.handshakeError.

package eap

import (
	"strings"
	"testing"
)

// eapTLSExchange is what driving one EAP-TLS conversation to its end revealed.
type eapTLSExchange struct {
	// serverSent holds every packet the AUTHENTICATOR put on the wire, in order.
	serverSent []*Packet
	// peerSent holds every packet the PEER put on the wire, in order.
	peerSent []*Packet
	// alertAt indexes peerSent at the round in which the peer's TLS engine
	// recorded its handshake failure, which is the round carrying its fatal
	// alert. -1 when the peer never failed.
	alertAt int
	// failureAt indexes serverSent at the first EAP-Failure. -1 when none.
	failureAt int
	// successAt indexes serverSent at the first EAP-Success. -1 when none.
	successAt int
	// peerErr is the peer's first reported error.
	peerErr error
}

// driveEAPTLSConversation runs the real authenticator Session against the real
// peer and records both sides of the wire.
//
// The peer's alert round is identified by ps.tlsErr. The peer's own handshake
// goroutine sets it, and nothing else writes it.
//
// A search of the response for a TLS alert content type cannot do that job.
// Under TLS 1.3 the peer rejects the chain once handshake keys are in force, so
// the alert is sealed in a record whose OUTER type is 23. Its legitimate
// encrypted flights carry that same type. The assertions below still check that
// the round carried a TLS record and not a bare fragment ACK.
func driveEAPTLSConversation(t *testing.T, serverCfg MethodConfig, peer *PeerSession, maxRounds int) *eapTLSExchange {
	t.Helper()

	sess, err := NewSession(TypeTLS, serverCfg)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	ex := &eapTLSExchange{alertAt: -1, failureAt: -1, successAt: -1}
	req := sess.Begin()

	for range maxRounds {
		ex.serverSent = append(ex.serverSent, req)
		if req.Code == CodeFailure && ex.failureAt < 0 {
			ex.failureAt = len(ex.serverSent) - 1
		}
		if req.Code == CodeSuccess && ex.successAt < 0 {
			ex.successAt = len(ex.serverSent) - 1
		}

		pres := peer.Process(req)
		if pres.Response != nil {
			ex.peerSent = append(ex.peerSent, pres.Response)
			if ex.alertAt < 0 && peer.tlsErr.Load() != nil {
				ex.alertAt = len(ex.peerSent) - 1
			}
		}
		if pres.Err != nil {
			ex.peerErr = pres.Err
			break
		}
		if pres.Done || pres.Response == nil {
			break
		}

		next := sess.Process(pres.Response)
		if next == nil {
			break
		}
		req = next
	}
	return ex
}

// TestRFC5216ServerRepliesEAPFailureToPeerAlert drives an exchange in which the
// PEER refuses the authenticator's certificate chain and asserts the
// authenticator answers the peer's TLS alert with EAP-Failure.
//
// RFC requirement: RFC5216-2.1.3-1 positive -- RFC 5216 Section 2.1.3: "The EAP
// Server MUST reply with an EAP-Failure packet since server authentication
// failure is a terminal condition." The peer sends its fatal alert as an
// EAP-Response, and the very next packet the authenticator puts on the wire is
// EAP-Failure.
//
// VALIDATES: the peer's alert reaches the authenticator as an EAP-Response
// carrying a TLS record, tlsMethod.Process reads the resulting handshake error,
// and Session.handleMethod turns it into an EAP-Failure packet.
// PREVENTS: the authenticator answering the alert with a bare fragment ACK,
// leaving a peer that has already given up waiting for a reply that never comes.
func TestRFC5216ServerRepliesEAPFailureToPeerAlert(t *testing.T) {
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

	ex := driveEAPTLSConversation(t, serverCfg, peer, 40)

	if ex.alertAt < 0 {
		t.Fatalf("the peer never rejected the authenticator across %d of its packets "+
			"(peerErr=%v): this test needs the PEER to be the side that refuses",
			len(ex.peerSent), ex.peerErr)
	}

	// The alert round must carry a TLS record, not the bare fragment ACK a round
	// with nothing to say produces. Without it the authenticator's TLS engine
	// never sees the rejection and has no error to report.
	alert := ex.peerSent[ex.alertAt]
	if alert.Code != CodeResponse {
		t.Errorf("the peer's alert went out as code %d, want %d (EAP-Response)", alert.Code, CodeResponse)
	}
	if td := alert.TypeData; len(td) == 1 && td[0] == 0 {
		t.Fatal("the peer answered with a bare fragment ACK instead of its fatal TLS alert: " +
			"the authenticator's TLS engine learns nothing and has no failure to report")
	}

	if ex.failureAt < 0 {
		t.Fatalf("the authenticator never sent EAP-Failure across %d packets (peerErr=%v): "+
			"RFC 5216 Section 2.1.3 makes server authentication failure a terminal condition "+
			"the EAP Server MUST answer with EAP-Failure", len(ex.serverSent), ex.peerErr)
	}
	if ex.successAt >= 0 {
		t.Errorf("the authenticator sent EAP-Success (packet %d) for a conversation the peer rejected", ex.successAt)
	}

	// The reply must be the answer to the alert, not a later packet. The peer
	// stops waiting once it has one reply, so a bare ACK in between is a round
	// the peer no longer answers. The loop appends one packet from each side per
	// round, so serverSent[i+1] answers peerSent[i].
	if want := ex.alertAt + 1; ex.failureAt != want {
		t.Errorf("the authenticator answered the peer's alert (its packet %d) with server packet %d, "+
			"but sent EAP-Failure only at packet %d: the alert must be answered with the failure itself",
			ex.alertAt, want, ex.failureAt)
	}

	// The peer must be able to tell the operator WHY, so its own error names the
	// chain failure rather than the generic "the authenticator sent Failure".
	if ex.peerErr == nil {
		t.Fatal("the peer reported no error for a certificate chain it refused")
	}
	if msg := ex.peerErr.Error(); !strings.Contains(msg, "verification failed") {
		t.Errorf("peer error %q does not name the chain verification failure that caused the rejection", msg)
	}
}

// TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate is the boundary of
// the two termination MUSTs: neither of them fires on a conversation nobody
// rejected.
//
// RFC requirement: RFC5216-2.1.3-1 negative -- the EAP-Failure is owed to a
// SERVER authentication failure. The peer accepts this server certificate, so it
// sends no TLS alert and the authenticator sends no EAP-Failure. The obligation
// answers the rejection, never an EAP-Response on its own.
//
// RFC requirement: RFC5216-2.1.3-4 negative -- "terminate the conversation"
// belongs to the reply that follows the authenticator's own alert. The
// authenticator sent no alert here, so it ends nothing. The conversation runs on
// to EAP-Success.
//
// VALIDATES: a mutually authenticated exchange produces exactly one EAP-Success,
// no EAP-Failure, and no peer-side TLS failure.
// PREVENTS: a fix that reaches EAP-Failure unconditionally -- an authenticator
// that failed every conversation would satisfy both positive tests and refuse
// every legitimate peer.
func TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	ex := driveEAPTLSConversation(t, pki.serverConfig(), peer, 40)

	if ex.peerErr != nil {
		t.Fatalf("the peer failed a handshake both sides should accept: %v", ex.peerErr)
	}
	if ex.alertAt >= 0 {
		t.Fatalf("the peer's TLS engine failed at its packet %d on a certificate it trusts", ex.alertAt)
	}
	if ex.failureAt >= 0 {
		t.Errorf("the authenticator sent EAP-Failure (packet %d of %d) for a peer it authenticated: "+
			"the RFC 5216 Section 2.1.3 failure is owed to a rejection, not to every conversation",
			ex.failureAt, len(ex.serverSent))
	}
	if ex.successAt < 0 {
		t.Fatalf("the authenticator never sent EAP-Success across %d packets", len(ex.serverSent))
	}
}
