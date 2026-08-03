// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- EAP-TLS successful termination
// RFC: rfc/short/rfc5216.md -- EAP-TLS termination (Section 2.1.3), success direction
//
// RFC 5216 Section 2.1.3 ends a SUCCESSFUL EAP-TLS conversation in three owed
// packets, one after the other:
//
//	"If the peer authenticates successfully, the EAP server MUST respond with an
//	EAP-Request packet with EAP-Type=EAP-TLS, which includes, in the case of a new
//	TLS session, one or more TLS records containing TLS change_cipher_spec and
//	finished handshake messages."
//
//	"If the EAP server authenticates successfully, the peer MUST send an
//	EAP-Response packet of EAP-Type=EAP-TLS, and no data.  The EAP Server then
//	MUST respond with an EAP-Success message."
//
// rfc5216_termination_test.go and eap_tls_alert_flight_test.go pin the FAILURE
// direction. This file pins the success direction, which no test reached: the
// existing handshake tests assert that EAP-Success arrives and that the two MSKs
// match, and neither of those sees the three packets the section names.
//
// These tests cap the AUTHENTICATOR at TLS 1.2, which is the version range this
// paragraph describes. The closing flight it names is a TLS 1.2 flight: under
// TLS 1.3 the server sends its Finished before it reads the client's, so it has
// nothing left to send afterwards, and EAP-TLS over TLS 1.3 is governed by RFC
// 9190 instead. Ze negotiates TLS 1.3 by default and accepts TLS 1.2 down to its
// MinVersion, so the capped exchange is a configuration a real peer reaches, not
// a shape invented for the test. Only the test's own tls.Config copy is touched;
// no production default changes.
//
// VALIDATES: after the peer's certificate is accepted the authenticator sends an
// EAP-Request/EAP-TLS carrying change_cipher_spec and Finished records, the peer
// answers with an EAP-Response/EAP-TLS carrying no data, and the authenticator
// then sends EAP-Success.
// PREVENTS: the authenticator concluding in the round that completes the
// handshake and dropping its closing flight -- the peer then waits for a server
// Finished that never arrives, its MSK stays zero while the authenticator's does
// not, and the two IKEv2 AUTH payloads (RFC 7296 Section 2.16) are computed over
// keys the ends do not share.

package eap

import (
	"crypto/tls"
	"encoding/binary"
	"testing"
)

// tlsRecordChangeCipherSpec is the TLS record content type of a
// change_cipher_spec record. The handshake content type that follows it in the
// closing flight is tlsRecordHandshake, declared in eap_tls_flight_test.go.
const tlsRecordChangeCipherSpec byte = 0x14

// eapTLSFlight is what driving one EAP-TLS conversation revealed, packet by
// packet. serverSent[i] is answered by peerSent[i]: the loop appends one packet
// from each side per round, and stops appending on the peer's side as soon as
// the peer concludes.
type eapTLSFlight struct {
	serverSent []*Packet
	peerSent   []*Packet

	peerDone  bool
	peerMSK   [64]byte
	serverMSK [64]byte
	peerErr   error

	// successAt and failureAt index serverSent at the first EAP-Success and the
	// first EAP-Failure. -1 when the conversation carried none.
	successAt int
	failureAt int
}

// driveEAPTLSFlight runs the real authenticator Session against the real peer and
// records both sides of the wire, with the authenticator's TLS version capped.
//
// The cap is applied to the method's own tls.Config before Begin, which is the
// call that starts the TLS engine goroutine. A cap set later would be read after
// the handshake had already begun and would do nothing.
func driveEAPTLSFlight(t *testing.T, cfg MethodConfig, peer *PeerSession, maxTLSVersion uint16, maxRounds int) *eapTLSFlight {
	t.Helper()

	sess, err := NewSession(TypeTLS, cfg)
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
	method.tlsConfig.MaxVersion = maxTLSVersion

	fl := &eapTLSFlight{successAt: -1, failureAt: -1}
	req := sess.Begin()

	for range maxRounds {
		fl.serverSent = append(fl.serverSent, req)
		switch {
		case req.Code == CodeSuccess && fl.successAt < 0:
			fl.successAt = len(fl.serverSent) - 1
			fl.serverMSK = sess.MSK()
		case req.Code == CodeFailure && fl.failureAt < 0:
			fl.failureAt = len(fl.serverSent) - 1
		}

		pres := peer.Process(req)
		if pres.Response != nil {
			fl.peerSent = append(fl.peerSent, pres.Response)
		}
		if pres.Err != nil {
			fl.peerErr = pres.Err
			break
		}
		if pres.Done {
			fl.peerDone = true
			fl.peerMSK = pres.MSK
			break
		}
		if pres.Response == nil {
			break
		}

		next := sess.Process(pres.Response)
		if next == nil {
			break
		}
		req = next
	}

	// Read the negotiated version ONLY once the handshake goroutine has returned.
	// tls.Conn.ConnectionState takes the handshake mutex, and HandshakeContext
	// holds it for as long as it is parked in eapTLSTransport.Read -- which is
	// where it sits whenever the peer stopped answering mid-handshake. Reading it
	// unconditionally deadlocks the test on exactly the exchanges that end early.
	if method.handshaked.Load() && maxTLSVersion != 0 {
		if v := method.conn.ConnectionState().Version; v > maxTLSVersion {
			t.Fatalf("authenticator negotiated TLS %#04x above the %#04x cap this test needs", v, maxTLSVersion)
		}
	}
	return fl
}

// tlsRecordContentTypes decodes an EAP-TLS TypeData that carries a complete,
// length-prefixed TLS message and returns each record's content type in order.
//
// It reports nil for a message that is not a complete first-and-only fragment
// (no L flag, an M flag, or a declared length the payload does not match), so a
// caller can tell "these are the records" from "this is not a whole message".
func tlsRecordContentTypes(td []byte) []byte {
	if len(td) < 1+4+tlsRecordHeaderLen || td[0]&eapTLSFlagL == 0 || td[0]&eapTLSFlagM != 0 {
		return nil
	}
	body := td[5:]
	if int(binary.BigEndian.Uint32(td[1:5])) != len(body) {
		return nil
	}

	var types []byte
	for off := 0; off+tlsRecordHeaderLen <= len(body); {
		end := off + tlsRecordHeaderLen + int(binary.BigEndian.Uint16(body[off+3:off+5]))
		if end > len(body) {
			return nil
		}
		types = append(types, body[off])
		off = end
	}
	return types
}

// bareEAPTLSResponse reports whether a packet is the "EAP-Response packet of
// EAP-Type=EAP-TLS, and no data" of RFC 5216 Section 2.1.3: the single flags
// octet, with no flag set and no TLS payload behind it.
func bareEAPTLSResponse(p *Packet) bool {
	return p != nil && p.Code == CodeResponse && p.Type == TypeTLS &&
		len(p.TypeData) == 1 && p.TypeData[0] == 0
}

// TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess drives a mutually
// authenticated EAP-TLS conversation over TLS 1.2 and asserts the three closing
// packets RFC 5216 Section 2.1.3 owes, in order.
//
// RFC requirement: RFC5216-2.1.3-5 positive -- RFC 5216 Section 2.1.3: "If the
// peer authenticates successfully, the EAP server MUST respond with an
// EAP-Request packet with EAP-Type=EAP-TLS, which includes, in the case of a new
// TLS session, one or more TLS records containing TLS change_cipher_spec and
// finished handshake messages." The packet before EAP-Success is that
// EAP-Request, and its TLS records are a change_cipher_spec followed by the
// handshake record carrying Finished.
//
// RFC requirement: RFC5216-2.1.3-7 positive -- RFC 5216 Section 2.1.3: "If the
// EAP server authenticates successfully, the peer MUST send an EAP-Response
// packet of EAP-Type=EAP-TLS, and no data." The peer answers that closing flight
// with the bare flags octet and nothing else.
//
// RFC requirement: RFC5216-2.1.3-8 positive -- RFC 5216 Section 2.1.3: "The EAP
// Server then MUST respond with an EAP-Success message." The packet after the
// peer's no-data response is EAP-Success.
//
// VALIDATES: the closing flight, the no-data acknowledgment, and EAP-Success are
// three consecutive packets, and the exchange yields one identical, non-zero MSK
// on both sides -- so the flight really is the one that completed the handshake.
// PREVENTS: the authenticator returning Done in the round that sets handshaked
// and dropping the flight, and the peer answering that flight with TLS data or
// with nothing at all.
func TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, 40)

	if fl.peerErr != nil {
		t.Fatalf("the peer failed a handshake both sides should accept: %v", fl.peerErr)
	}
	if fl.successAt < 0 {
		t.Fatalf("the authenticator never sent EAP-Success across %d packets", len(fl.serverSent))
	}
	if fl.successAt < 1 {
		t.Fatal("EAP-Success was the first packet of the conversation")
	}

	// RFC5216-2.1.3-5: the packet before EAP-Success is the closing flight.
	flight := fl.serverSent[fl.successAt-1]
	if flight.Code != CodeRequest || flight.Type != TypeTLS {
		t.Fatalf("the packet before EAP-Success is code %d type %d, want an EAP-Request (%d) with "+
			"EAP-Type=EAP-TLS (%d): RFC 5216 Section 2.1.3 owes the peer the server's closing flight",
			flight.Code, flight.Type, CodeRequest, TypeTLS)
	}
	if len(flight.TypeData) <= 1 {
		t.Fatal("the authenticator sent a bare fragment ACK in place of its closing flight: the peer " +
			"never receives the server Finished it needs to authenticate the server")
	}
	types := tlsRecordContentTypes(flight.TypeData)
	if len(types) < 2 {
		t.Fatalf("the closing flight decodes to %d TLS record(s) (%v), want change_cipher_spec "+
			"followed by the handshake record carrying Finished", len(types), types)
	}
	if types[0] != tlsRecordChangeCipherSpec {
		t.Errorf("the closing flight's first TLS record has content type %d, want %d "+
			"(change_cipher_spec)", types[0], tlsRecordChangeCipherSpec)
	}
	if types[1] != tlsRecordHandshake {
		t.Errorf("the closing flight's second TLS record has content type %d, want %d (handshake, "+
			"carrying Finished)", types[1], tlsRecordHandshake)
	}

	// RFC5216-2.1.3-7: the peer answers that flight with no data at all.
	if len(fl.peerSent) < fl.successAt {
		t.Fatalf("the peer sent %d packets and never answered the closing flight", len(fl.peerSent))
	}
	ack := fl.peerSent[fl.successAt-1]
	if !bareEAPTLSResponse(ack) {
		t.Errorf("the peer answered the closing flight with code %d type %d and %d octets of "+
			"TypeData (%v); RFC 5216 Section 2.1.3 asks for an EAP-Response of EAP-Type=EAP-TLS "+
			"and no data", ack.Code, ack.Type, len(ack.TypeData), ack.TypeData)
	}

	// RFC5216-2.1.3-8: and the authenticator answers that with EAP-Success.
	if code := fl.serverSent[fl.successAt].Code; code != CodeSuccess {
		t.Errorf("the packet after the peer's no-data response is code %d, want %d (EAP-Success)", code, CodeSuccess)
	}

	// The flight is the one that completed the handshake, not an unrelated packet
	// that happens to sit in that slot.
	var zero [64]byte
	if !fl.peerDone {
		t.Error("the peer did not conclude on EAP-Success")
	}
	if fl.peerMSK == zero || fl.serverMSK == zero {
		t.Errorf("an MSK is all zero (peer zero=%v, authenticator zero=%v): the closing flight did "+
			"not complete the handshake on both sides", fl.peerMSK == zero, fl.serverMSK == zero)
	}
	if fl.peerMSK != fl.serverMSK {
		t.Errorf("MSK mismatch after the closing flight:\n peer=  %x\n server=%x", fl.peerMSK, fl.serverMSK)
	}
}

// TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected is the boundary of the
// two authenticator-side success MUSTs: neither fires for a peer the
// authenticator refused.
//
// RFC requirement: RFC5216-2.1.3-5 negative -- the closing flight is owed "if the
// peer authenticates successfully". This peer's certificate chains to a CA the
// authenticator does not trust, so no EAP-Request it sends carries a
// change_cipher_spec record: the conversation ends on the alert and EAP-Failure
// instead.
//
// RFC requirement: RFC5216-2.1.3-8 negative -- EAP-Success answers the peer's
// no-data response at the end of a successful handshake. There is no successful
// handshake here, so the authenticator sends EAP-Failure and never EAP-Success.
//
// VALIDATES: a rejected peer receives no change_cipher_spec flight and no
// EAP-Success, and does receive EAP-Failure.
// PREVENTS: a fix that emits the closing flight or EAP-Success unconditionally --
// an authenticator that concluded every conversation would satisfy both positive
// assertions above while authenticating nobody.
func TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected(t *testing.T) {
	pki := newEAPTLSPKI(t)
	// The peer trusts the authenticator's CA, so it does not reject the server
	// first; its own client certificate is the untrusted one.
	peer := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
		CertPEM:   pki.untrustedClientCertPEM,
		KeyPEM:    pki.untrustedClientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, 40)

	if fl.successAt >= 0 {
		t.Errorf("the authenticator sent EAP-Success (packet %d) for a client certificate signed by "+
			"an untrusted CA", fl.successAt)
	}
	if fl.failureAt < 0 {
		t.Fatalf("the authenticator never sent EAP-Failure across %d packets (peerErr=%v)",
			len(fl.serverSent), fl.peerErr)
	}
	for i, p := range fl.serverSent {
		if p.Code != CodeRequest || p.Type != TypeTLS {
			continue
		}
		for _, ct := range tlsRecordContentTypes(p.TypeData) {
			if ct == tlsRecordChangeCipherSpec {
				t.Errorf("the authenticator's packet %d carries a change_cipher_spec record: it sent "+
					"the closing flight to a peer it could not authenticate", i)
			}
		}
	}
}

// TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse is the boundary of the
// peer-side success MUST: the no-data response belongs to a server the peer
// authenticated, and to nothing else.
//
// RFC requirement: RFC5216-2.1.3-7 negative -- the no-data EAP-Response is owed
// "if the EAP server authenticates successfully". This authenticator presents a
// certificate the peer cannot chain to its trust anchor, so the peer's last
// packet carries its fatal TLS alert rather than the empty acknowledgment, and no
// EAP-Success follows.
//
// VALIDATES: a peer that refuses the authenticator answers with TLS data, not
// with the bare flags octet.
// PREVENTS: readAndSendTLS degenerating to the empty response for every round --
// the positive test would still see a no-data packet in the right slot, while the
// authenticator's TLS engine would never receive the alert and would have no
// failure to report (the bare-ACK-until-the-reaper defect).
func TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse(t *testing.T) {
	pki := newEAPTLSPKI(t)
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

	fl := driveEAPTLSFlight(t, serverCfg, peer, tls.VersionTLS12, 40)

	if fl.successAt >= 0 {
		t.Fatalf("the authenticator sent EAP-Success (packet %d) for a server certificate the peer "+
			"cannot verify: this test needs the PEER to be the side that refuses", fl.successAt)
	}
	if peer.tlsErr.Load() == nil {
		t.Fatalf("the peer's TLS engine recorded no failure across %d of its packets (peerErr=%v)",
			len(fl.peerSent), fl.peerErr)
	}
	if len(fl.peerSent) == 0 {
		t.Fatal("the peer sent nothing at all")
	}

	last := fl.peerSent[len(fl.peerSent)-1]
	if bareEAPTLSResponse(last) {
		t.Errorf("the peer's last packet is the no-data EAP-Response RFC 5216 Section 2.1.3 owes a "+
			"server it authenticated; it refused this one, so that packet must carry its fatal TLS "+
			"alert instead (%d packets sent, peerErr=%v)", len(fl.peerSent), fl.peerErr)
	}
}
