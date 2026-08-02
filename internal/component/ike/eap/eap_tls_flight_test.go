// RFC: rfc/short/rfc5216.md -- EAP-TLS fragmentation and the empty-ACK rule (Section 2.1.5)
//
// The first EAP-TLS response each side sends must carry that side's opening TLS
// flight. Both sides used to snapshot the TLS engine's output buffer with no
// wait, so the snapshot ran before the engine had written anything and the EAP
// layer sent a bare fragment ACK instead. RFC 5216 Section 2.1.5 permits an
// empty EAP-TLS message only in answer to a message carrying the M flag, so a
// conforming authenticator refuses it: strongSwan logs "EAP method EAP_TLS
// failed" before any TLS record crosses.
//
// VALIDATES: the peer's answer to an EAP-TLS Start carries a real ClientHello
// record, and the authenticator's answer to that ClientHello carries a real
// ServerHello record.
// PREVENTS: regressing either side back to an unsynchronised buffer snapshot,
// which emits a spurious empty-ACK and fails the method against any conforming
// peer. Governing requirement: RFC5216-2.1.5-1.

package eap

import (
	"testing"
)

// TLS record and handshake message tags (RFC 8446 Section 5.1, Section 4).
const (
	tlsRecordHandshake  byte = 0x16
	tlsHandshakeCliHlo  byte = 0x01
	tlsHandshakeSrvHlo  byte = 0x02
	tlsRecordHeaderSize      = 5
)

// tlsBytesFromTypeData strips the EAP-TLS header from an EAP-TLS TypeData field
// and returns the TLS bytes it carries. It returns nil when the message carries
// no TLS data, which is exactly the bare fragment ACK this file exists to catch.
func tlsBytesFromTypeData(td []byte) []byte {
	if len(td) == 0 {
		return nil
	}
	off := 1
	if td[0]&eapTLSFlagL != 0 {
		if len(td) < 5 {
			return nil
		}
		off = 5
	}
	if off >= len(td) {
		return nil
	}
	return td[off:]
}

// assertHandshakeRecord fails unless b opens with a TLS handshake record whose
// first handshake message has the wanted type.
func assertHandshakeRecord(t *testing.T, side string, td []byte, want byte) {
	t.Helper()

	body := tlsBytesFromTypeData(td)
	if len(body) == 0 {
		t.Fatalf("%s sent an empty EAP-TLS message (TypeData %#v): RFC 5216 Section 2.1.5 allows a bare "+
			"fragment ACK only in answer to a message with the M flag, and none was received", side, td)
	}
	if body[0] != tlsRecordHandshake {
		t.Fatalf("%s: TLS content type is 0x%02x, want 0x%02x (handshake)", side, body[0], tlsRecordHandshake)
	}
	if len(body) <= tlsRecordHeaderSize {
		t.Fatalf("%s: TLS record carries no handshake message (%d bytes)", side, len(body))
	}
	if got := body[tlsRecordHeaderSize]; got != want {
		t.Fatalf("%s: handshake message type is 0x%02x, want 0x%02x", side, got, want)
	}
}

// eapTLSStart is the authenticator's EAP-TLS Start request (S flag, no TLS data).
func eapTLSStart() *Packet {
	return &Packet{Code: CodeRequest, Identifier: 1, Type: TypeTLS, TypeData: []byte{eapTLSFlagS}}
}

// TestEAPTLSPeerFirstResponseCarriesClientHello drives the peer's answer to an
// EAP-TLS Start and asserts it carries a ClientHello.
//
// Without the transport's output wakeup this is deterministically red: the peer
// starts the TLS handshake on a goroutine and then snapshots the output buffer,
// which cannot yet hold the ClientHello, so TypeData is []byte{0}.
func TestEAPTLSPeerFirstResponseCarriesClientHello(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})
	// This test stops after the first flight, so the peer's TLS engine stays
	// parked in eapTLSTransport.Read until the transport is closed.
	t.Cleanup(peer.Close)

	res := peer.Process(eapTLSStart())
	if res.Err != nil {
		t.Fatalf("peer refused the EAP-TLS Start: %v", res.Err)
	}
	if res.Response == nil {
		t.Fatal("peer produced no response to the EAP-TLS Start")
	}
	if res.Response.Code != CodeResponse || res.Response.Type != TypeTLS {
		t.Fatalf("peer replied with code %d type %d, want code %d type %d",
			res.Response.Code, res.Response.Type, CodeResponse, TypeTLS)
	}
	assertHandshakeRecord(t, "peer", res.Response.TypeData, tlsHandshakeCliHlo)
}

// TestEAPTLSAuthenticatorFirstResponseCarriesServerHello feeds the peer's real
// ClientHello to the authenticator and asserts the reply carries a ServerHello.
//
// This is the sibling call site of the peer defect: tlsMethod.Process took the
// same unsynchronised snapshot, so it answered a ClientHello with a bare ACK.
func TestEAPTLSAuthenticatorFirstResponseCarriesServerHello(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})
	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("create authenticator session: %v", err)
	}
	// This test stops after the ServerHello, so both TLS engines stay parked in
	// eapTLSTransport.Read until their transports are closed.
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	// RFC 3748 Section 5.1: the authenticator opens with an Identity request, so
	// the EAP-TLS Start is the reply to the peer's Identity response.
	identity := peer.Process(sess.Begin())
	if identity.Err != nil {
		t.Fatalf("peer refused the Identity request: %v", identity.Err)
	}
	start := sess.Process(identity.Response)
	if start == nil || start.Type != TypeTLS {
		t.Fatalf("authenticator produced %+v, want an EAP-TLS Start", start)
	}
	if len(start.TypeData) == 0 || start.TypeData[0]&eapTLSFlagS == 0 {
		t.Fatalf("EAP-TLS Start TypeData %#v does not carry the S flag", start.TypeData)
	}

	hello := peer.Process(start)
	if hello.Err != nil {
		t.Fatalf("peer refused the EAP-TLS Start: %v", hello.Err)
	}
	if hello.Response == nil {
		t.Fatal("peer produced no ClientHello response")
	}

	// RFC 5216 Section 2.1.5: while the peer's fragment carries the M flag the
	// authenticator MUST answer with a bare ACK and the peer sends the next
	// fragment. The reply to the LAST fragment is the one that must carry the
	// ServerHello, so walk any fragmentation rather than assuming one message.
	resp := hello.Response
	for round := 1; ; round++ {
		if round > maxEAPRounds {
			t.Fatalf("authenticator never answered the ClientHello with TLS data in %d rounds", maxEAPRounds)
		}

		more := len(resp.TypeData) > 0 && resp.TypeData[0]&eapTLSFlagM != 0

		reply := sess.Process(resp)
		if reply == nil {
			t.Fatal("authenticator produced no reply to the ClientHello")
		}
		if reply.Code != CodeRequest || reply.Type != TypeTLS {
			t.Fatalf("authenticator replied with code %d type %d, want code %d type %d",
				reply.Code, reply.Type, CodeRequest, TypeTLS)
		}

		if !more {
			// The peer's flight is complete, so this reply must carry the
			// ServerHello. A bare ACK here is the defect this test exists for.
			assertHandshakeRecord(t, "authenticator", reply.TypeData, tlsHandshakeSrvHlo)
			return
		}

		// A fragment ACK is the correct answer to an M-flagged fragment, and it
		// must carry no TLS data.
		if body := tlsBytesFromTypeData(reply.TypeData); len(body) != 0 {
			t.Fatalf("authenticator answered an M-flagged fragment with %d bytes of TLS data, want a bare ACK", len(body))
		}

		next := peer.Process(reply)
		if next.Err != nil {
			t.Fatalf("peer failed to send its next fragment: %v", next.Err)
		}
		if next.Response == nil {
			t.Fatal("peer produced no next fragment")
		}
		resp = next.Response
	}
}
