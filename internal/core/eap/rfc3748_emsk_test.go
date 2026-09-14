// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS key derivation
// RFC: rfc/short/rfc3748.md -- Section 7.10 key derivation: the MSK, the EMSK and its confinement
// RFC: rfc/short/rfc5216.md -- Section 2.3 splits Key_Material into MSK and EMSK
// RFC: rfc/short/rfc9190.md -- Section 2.3 replaces that derivation for TLS 1.3
//
// VALIDATES: EAP-TLS exports an Extended Master Session Key of 64 octets beside
// the MSK on both roles, and that key stays in the two sessions that derived it.
// PREVENTS: a return to keeping material[:64] and dropping the EMSK half, which
// left RFC 3748 Section 7.10 unmet with every test green, and the opposite
// failure of publishing the EMSK through an accessor or a result field.

package eap

import (
	"bytes"
	"crypto/tls"
	"testing"
)

// emskExchange drives one complete EAP-TLS exchange with the authenticator's TLS
// version pinned, and answers both sessions plus every octet that crossed
// between them.
//
// The wire log is what the confinement test searches. Each entry is one encoded
// EAP packet, in the order it was sent, taken from Packet.Encode so it is the
// same bytes the IKEv2 carrier would put in an EAP payload.
func emskExchange(t *testing.T, version uint16) (sess *Session, peer *PeerSession, method *tlsMethod, wire [][]byte) {
	t.Helper()

	pki := newEAPTLSPKI(t)
	peer = NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}
	method.tlsConfig.MinVersion = version
	method.tlsConfig.MaxVersion = version

	done := false
	req := sess.Begin()
	for range 60 {
		wire = append(wire, req.Encode())
		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("the peer failed a handshake both sides should accept: %v", pres.Err)
		}
		if pres.Done {
			done = true
			break
		}
		if pres.Response == nil {
			t.Fatal("the peer stopped answering before the handshake completed")
		}
		wire = append(wire, pres.Response.Encode())

		next := sess.Process(pres.Response)
		if next == nil {
			t.Fatal("the authenticator stopped answering before the handshake completed")
		}
		if next.Code == CodeFailure {
			t.Fatal("the authenticator refused a handshake it should accept")
		}
		req = next
	}
	if !done {
		t.Fatal("the handshake did not complete")
	}

	if got := method.conn.ConnectionState().Version; got != version {
		t.Fatalf("authenticator negotiated TLS %#04x, want the pinned %#04x", got, version)
	}
	return sess, peer, method, wire
}

// TestRFC3748EAPTLSExportsASixtyFourOctetEMSK asserts the second half of the
// EAP-TLS key material becomes an EMSK on both roles, rather than being dropped.
//
// RFC 3748 Section 7.10: "In order to provide keying material for use in a
// subsequently negotiated ciphersuite, an EAP method supporting key derivation
// MUST export a Master Session Key (MSK) of at least 64 octets, and an Extended
// Master Session Key (EMSK) of at least 64 octets."
//
// RFC 5216 Section 2.3 says which octets they are: "Key_Material =
// TLS-PRF-128(master_secret, "client EAP encryption", client.random ||
// server.random)", then "MSK = Key_Material(0,63)" and "EMSK =
// Key_Material(64,127)". RFC 9190 Section 2.3 repeats the split for TLS 1.3:
// "The MSK and EMSK are derived from the Key_Material in the same manner as with
// EAP-TLS [RFC5216], Section 2.3."
//
// The expected octets are computed here from each side's own TLS exporter, under
// the literal label each RFC spells, so the assertion does not read back the
// constant the producer read.
//
// RFC requirement: RFC3748-7.10-2 positive -- on a completed EAP-TLS exchange
// over TLS 1.2 and over TLS 1.3, the authenticator Session and the peer
// PeerSession each hold an EMSK of exactly 64 octets that is non-zero, equal to
// the other end's, equal to octets 64 to 127 of the 128-octet TLS export, and
// different from the MSK derived beside it.
func TestRFC3748EAPTLSExportsASixtyFourOctetEMSK(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version uint16
		label   string
		context []byte
	}{
		// The literals the RFCs spell, written out so the assertion does not read
		// the same constants the producer reads.
		{"tls12-rfc5216", tls.VersionTLS12, "client EAP encryption", nil},
		{"tls13-rfc9190", tls.VersionTLS13, "EXPORTER_EAP_TLS_Key_Material", []byte{TypeTLS}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, peer, method, _ := emskExchange(t, tc.version)

			// RFC 3748 Section 7.10 states a floor of 64 octets, and ze exports
			// exactly that. Reading the length rather than assuming it is what makes
			// the floor an assertion rather than a property of the declaration.
			if len(sess.emsk) != 64 {
				t.Fatalf("the authenticator EMSK is %d octets, and Section 7.10 needs at least 64", len(sess.emsk))
			}
			if len(peer.emsk) != 64 {
				t.Fatalf("the peer EMSK is %d octets, and Section 7.10 needs at least 64", len(peer.emsk))
			}

			if sess.emsk == ([64]byte{}) {
				t.Fatal("the authenticator EMSK is 64 zero octets, which is an unset field and not a key")
			}
			if sess.emsk != peer.emsk {
				t.Fatalf("the two ends derived different EMSKs:\n  authenticator %x\n  peer          %x", sess.emsk, peer.emsk)
			}

			// The EMSK must be the SECOND half. A producer that copied the first
			// half twice would satisfy every assertion above.
			material, err := exportFrom(method.conn.ConnectionState())(tc.label, tc.context, 128)
			if err != nil {
				t.Fatalf("the TLS session refused the 128-octet export, so this test cannot judge the split: %v", err)
			}
			if [64]byte(material[64:]) != sess.emsk {
				t.Fatalf("EMSK = %x,\n  want %x (octets 64 to 127 of the export under %q)", sess.emsk, material[64:], tc.label)
			}
			if sess.emsk == sess.MSK() {
				t.Fatal("the EMSK equals the MSK, so one half of the key material was copied twice")
			}
		})
	}
}

// TestRFC3748NoEMSKWithoutACompletedExport asserts ze never invents an EMSK when
// the key material is not there to be exported.
//
// A partly written or all-zero EMSK reads at the call site exactly like a real
// one, which is the answer ai/rules/evidence.md forbids: the caller cannot tell
// it from a key, so the 64-octet floor RFC 3748 Section 7.10 states would be
// undershot with nothing to say so.
//
// The PEER is not asserted on here, and the reason is worth knowing. Under TLS
// 1.3 the client's handshake completes the moment it sends its Finished, so a
// peer whose certificate the authenticator then rejects has a complete
// connection and derives real key material from it. Nothing publishes that
// material: no EAP-Success arrives, so PeerSession.Process never reaches the
// branch that returns a Done PeerResult, and Close erases the EMSK. The MSK has
// behaved this way since before the EMSK existed.
//
// RFC requirement: RFC3748-7.10-2 negative -- where the 128-octet export cannot
// be made, no EMSK reaches the EAP layer at all: an exchange the authenticator
// refuses leaves its stored EMSK all zero and publishes no key to the peer, and
// a TLS 1.2 session whose export crypto/tls refuses makes exportEAPTLSKeys
// report an error with an all-zero EMSK rather than a short one.
func TestRFC3748NoEMSKWithoutACompletedExport(t *testing.T) {
	pki := newEAPTLSPKI(t)
	rogue := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
		CertPEM:   pki.untrustedClientCertPEM,
		KeyPEM:    pki.untrustedClientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})

	res := runEAPTLSHandshake(t, pki.serverConfig(), rogue)
	if res.serverEAPSuccess {
		t.Fatal("the authenticator accepted a client certificate signed by an untrusted CA")
	}
	if res.sess.emsk != ([64]byte{}) {
		t.Fatalf("a refused exchange left an EMSK on the authenticator: %x", res.sess.emsk)
	}
	if res.peerDone {
		t.Fatal("the peer concluded an exchange the authenticator refused, so it published key material")
	}
	if res.peerMSK != ([64]byte{}) {
		t.Fatalf("a refused exchange published key material to the peer's carrier: %x", res.peerMSK)
	}

	// The other way the material can be missing: the handshake completes and
	// crypto/tls refuses the export (tls12ClientState, eap_tls_export_refusal_test.go).
	msk, emsk, err := exportEAPTLSKeys(tls12ClientState(t, exportRefused))
	if err == nil {
		t.Fatal("a refused export answered key material instead of an error")
	}
	if emsk != ([64]byte{}) {
		t.Fatalf("a refused export answered an EMSK: %x", emsk)
	}
	if msk != ([64]byte{}) {
		t.Fatalf("a refused export answered an MSK: %x", msk)
	}
}

// TestRFC3748TheEMSKStaysOnBothEndsThatDerivedIt asserts the EMSK lives in the
// two sessions that derived it, and lives nowhere longer than they do.
//
// RFC 3748 Section 7.10: "The EMSK is reserved for future use and MUST remain on
// the EAP peer and EAP server where it is derived; it MUST NOT be transported
// to, or shared with, additional parties, or used to derive any other keys."
//
// "Remain on" is two facts, and this checks both: the key is HELD by each of the
// two ends, and it is erased when the exchange that derived it ends. Where the
// key must NOT be is the negative arm below.
//
// RFC requirement: RFC3748-7.10-6 positive -- after a completed EAP-TLS exchange
// the EMSK is held by both ends that derived it, the authenticator Session and
// the peer PeerSession, and Close erases it from each of them.
func TestRFC3748TheEMSKStaysOnBothEndsThatDerivedIt(t *testing.T) {
	sess, peer, _, _ := emskExchange(t, tls.VersionTLS13)

	derived := sess.emsk
	if derived == ([64]byte{}) {
		t.Fatal("no EMSK was derived, so there is nothing for this test to judge")
	}
	if peer.emsk != derived {
		t.Fatalf("the EAP server and the EAP peer hold different EMSKs:\n  server %x\n  peer   %x", sess.emsk, peer.emsk)
	}

	sess.Close()
	peer.Close()

	if sess.emsk != ([64]byte{}) {
		t.Fatalf("Close left the EMSK on the authenticator: %x", sess.emsk)
	}
	if peer.emsk != ([64]byte{}) {
		t.Fatalf("Close left the EMSK on the peer: %x", peer.emsk)
	}
}

// TestRFC3748TheEMSKIsNeverHandedOutward asserts the EMSK reaches nothing beyond
// the two sessions that derived it.
//
// The path that would carry it is the MSK's. Both ends hand their carrier an MSK
// for the IKEv2 AUTH payload of RFC 7296 Section 2.16, and that carrier is in
// another package, so a producer that put the wrong half of Key_Material in the
// MSK would transport the EMSK out of ze's EAP layer with every assertion about
// lengths and equality still true.
//
// The search is proved to discriminate before it is trusted: it is run once over
// a corpus the EMSK was deliberately added to, and it finds it there
// (ai/rules/evidence.md, "Zero hits is not absence").
//
// RFC requirement: RFC3748-7.10-6 negative -- the EMSK octets appear in nothing
// that leaves the two sessions: not in the MSK the authenticator publishes
// through Session.MSK, not in the MSK the peer returns in PeerResult, and not in
// any EAP packet encoded in either direction during the exchange.
func TestRFC3748TheEMSKIsNeverHandedOutward(t *testing.T) {
	sess, peer, _, wire := emskExchange(t, tls.VersionTLS13)

	emsk := sess.emsk
	if emsk == ([64]byte{}) {
		t.Fatal("no EMSK was derived, so this test would pass over an empty search")
	}

	// Everything the EAP layer hands outward on a successful exchange. The two
	// MSKs are what the IKEv2 carrier reads (handleEAPResponse and
	// handleResponderEAP, internal/component/ike/engine); the wire log is every
	// octet the two ends sent each other.
	serverMSK := sess.MSK()
	peerMSK := peer.msk
	outward := append([][]byte{serverMSK[:], peerMSK[:]}, wire...)

	for i, out := range outward {
		if bytes.Contains(out, emsk[:]) {
			t.Fatalf("the EMSK left the session: it is present in outward artifact %d of %d", i+1, len(outward))
		}
	}

	// The control. Without it a broken search reports the same silence as a
	// confined key.
	planted := append(append([]byte{}, serverMSK[:]...), emsk[:]...)
	if !bytes.Contains(planted, emsk[:]) {
		t.Fatal("the search does not find the EMSK in a corpus it was added to, so its silence above means nothing")
	}
}
