// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS authenticator termination
// RFC: rfc/short/rfc5216.md -- EAP-TLS termination (Section 2.1.3)
//
// RFC 5216 Section 2.1.3 spends TWO rounds ending a rejected handshake, and the
// authenticator has to spend both. It SHOULD send the fatal TLS alert in an
// EAP-Request, it MUST then wait for the peer's EAP-Response, and only then MUST
// it send EAP-Failure. A round that carries the alert AND the failure carries
// neither: MethodResult holds both fields, but Session.handleMethod tests Err
// first and answers with s.failure(), so the Response is discarded and the alert
// octets never reach the wire.
//
// These tests pin the split. The file is separate from
// eap_tls_failure_report_test.go because that one asks whether the rejection is
// REPORTED at all; this one asks what the peer SEES while it is reported.

package eap

import (
	"encoding/binary"
	"strings"
	"testing"
)

// TLS record content types a fatal alert can arrive under, and the record header
// length that precedes the payload.
//
// Both content types are correct, and which one appears is a property of the
// negotiated version rather than of this code. Under TLS 1.2 the client
// certificate is rejected before the cipher change, so the alert is a plaintext
// record of type 21. Under TLS 1.3 the handshake keys are already in force when
// the client certificate arrives, so the same alert is sealed inside a record
// whose OUTER type is 23. Asserting 21 alone would pin the test to a TLS version
// nobody chose deliberately.
const (
	tlsRecordAlert           = 21
	tlsRecordApplicationData = 23
	tlsRecordHeaderLen       = 5
)

// alertFlight drives a real EAP-TLS Session against a real peer until the
// authenticator refuses the handshake, and returns the packet that carried the
// refusal to the peer.
//
// The stop condition is `Session.Err`, which the branch under test fills through
// MethodResult.FinalRequest and nothing else in this exchange writes. Stopping on
// `transport.handshakeError() != nil` instead would be merely CORRELATED with the
// alert round: it re-reads outside Process a value Process has already consumed,
// so if the 2s eapTLSSettleBackstop fires on a loaded host, Process can answer a
// bare ACK and the engine can record the error a moment later. This loop would
// then return a non-alert packet and the test would redden blaming a production
// defect that is not there.
//
// The peer is never given the returned packet. That is the point: it models the
// peer that abandons the exchange after the alert, which is what strongSwan does
// and what every assertion about the cause being RECORDED has to survive.
func alertFlight(t *testing.T, sess *Session, peer *PeerSession, maxRounds int) *Packet {
	t.Helper()

	req := sess.Begin()
	for i := range maxRounds {
		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("round %d: the peer failed before the authenticator rejected it (%v); "+
				"this test needs the AUTHENTICATOR to be the side that refuses", i+1, pres.Err)
		}
		if pres.Response == nil {
			t.Fatalf("round %d: the peer stopped answering before the rejection", i+1)
		}

		next := sess.Process(pres.Response)
		if next == nil {
			t.Fatalf("round %d: the authenticator discarded the peer's response", i+1)
		}
		if next.Code == CodeSuccess {
			t.Fatalf("round %d: the authenticator completed a handshake it was meant to reject", i+1)
		}
		if err := sess.Err(); err != nil {
			if next.Code == CodeFailure {
				t.Fatalf("round %d: the authenticator recorded its cause (%v) only on the round "+
					"that sends EAP-Failure, and the refused peer decides whether that round ever "+
					"happens. RFC 5216 Section 2.1.3 sends the fatal TLS alert first, and the "+
					"cause belongs to that round", i+1, err)
			}
			return next
		}
		req = next
	}
	t.Fatalf("the authenticator never rejected the peer within %d rounds", maxRounds)
	return nil
}

// TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure asserts the two
// rounds RFC 5216 Section 2.1.3 describes, in order.
//
// RFC requirement: RFC5216-2.1.3-3 positive -- RFC 5216 Section 2.1.3: "To ensure
// that the peer receives the TLS alert message, the EAP server MUST wait for the
// peer to reply with an EAP-Response packet." The round that produces the alert
// sends an EAP-Request and nothing else: no EAP-Failure and no EAP-Success reach
// the peer until the reply arrives, so the exchange is still open while it waits.
//
// RFC requirement: RFC5216-2.1.3-4 positive -- RFC 5216 Section 2.1.3: a reply
// that "contain[s] an EAP-Response packet with EAP-Type=EAP-TLS and no data" is
// answered thus: "the EAP-Server MUST send an EAP-Failure packet and terminate
// the conversation." The empty reply below draws exactly one packet, EAP-Failure.
//
// VALIDATES: the round that detects a rejected client certificate sends an
// EAP-Request carrying the TLS engine's fatal alert, records the certificate
// cause in that same round, and sends no EAP-Failure until the peer replies.
// PREVENTS: two defects at once. The alert being returned beside the error, where
// Session.handleMethod drops it and the peer is told only that the exchange ended
// -- the one thing Section 2.1.3 exists to stop ("so as to allow the peer to
// inform the user or log the cause of the failure"). And the cause being held
// back for the EAP-Failure round, which the refused peer decides whether to send.
func TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure(t *testing.T) {
	impostor := newImpostorPKI(t)

	sess, err := NewSession(TypeTLS, impostor.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	peer := NewPeerSessionTLS("impostor-pki-client", impostor.peerConfig())
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	alert := alertFlight(t, sess, peer, 40)

	// The alert round must carry a TLS record, not the bare fragment ACK that a
	// round with nothing to say produces.
	if alert.Code != CodeRequest {
		t.Errorf("the alert went out as code %d, want %d (EAP-Request): RFC 5216 Section 2.1.3 "+
			"asks for an EAP-Request packet with EAP-Type=EAP-TLS", alert.Code, CodeRequest)
	}
	td := alert.TypeData
	if len(td) == 1 && td[0] == 0 {
		t.Fatal("the authenticator sent a bare fragment ACK instead of the fatal TLS alert: " +
			"the peer learns that the exchange ended and never learns why")
	}
	if td[0]&eapTLSFlagM != 0 {
		t.Fatalf("the alert message sets the M flag (%#02x), so it is one fragment of a message "+
			"and not the whole alert", td[0])
	}
	// RFC 9190 Section 2.1.9 forbids the L bit on an unfragmented message, and an
	// alert is far short of a fragment, so the TLS record opens at offset 1. The
	// declared length is still checked wherever the L bit is set, because the same
	// sentence obliges this side to read both shapes.
	record := tlsBytesFromTypeData(td)
	if len(record) < tlsRecordHeaderLen {
		t.Fatalf("the alert message is %d octets with flags %#02x, want a whole TLS record "+
			"behind the EAP-TLS header", len(td), td[0])
	}
	if td[0]&eapTLSFlagL != 0 {
		if declared := int(binary.BigEndian.Uint32(td[1:5])); declared != len(record) {
			t.Errorf("the alert declares %d octets and carries %d", declared, len(record))
		}
	}
	if ct := record[0]; ct != tlsRecordAlert && ct != tlsRecordApplicationData {
		t.Errorf("the alert's TLS record content type is %d, want %d (alert) or %d "+
			"(an alert sealed under TLS 1.3 handshake keys)", ct, tlsRecordAlert, tlsRecordApplicationData)
	}

	// The cause is in hand on the SAME round, before the peer has answered
	// anything. A peer that walks away now leaves the operator this sentence.
	msg := sess.Err().Error()
	if !strings.Contains(msg, "unknown authority") {
		t.Errorf("the recorded cause %q does not carry the TLS engine's own reason", msg)
	}
	if strings.Contains(msg, "no MSK") {
		t.Errorf("the recorded cause %q reports the missing MSK rather than the certificate "+
			"failure that caused it", msg)
	}

	// RFC 5216 Section 2.1.3: "The EAP-Response packet sent by the peer ... MAY
	// contain an EAP-Response packet with EAP-Type=EAP-TLS and no data, in which
	// case the EAP-Server MUST send an EAP-Failure packet". This is that reply.
	last := sess.Process(&Packet{
		Code:       CodeResponse,
		Identifier: alert.Identifier,
		Type:       TypeTLS,
		TypeData:   []byte{0},
	})
	if last == nil {
		t.Fatal("the reply to the alert drew no packet at all, so the conversation never terminated")
	}
	if last.Code != CodeFailure {
		t.Errorf("the reply to the alert drew code %d, want %d (EAP-Failure)", last.Code, CodeFailure)
	}
	if sess.Succeeded() {
		t.Error("the session reports success for a client certificate it could not verify")
	}
	if got := sess.Err().Error(); got != msg {
		t.Errorf("the cause changed on the EAP-Failure round, from %q to %q", msg, got)
	}
}

// TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure drives the whole EAP
// exchange through `Session` and records what the PEER actually received.
//
// The defect this closes lived in `Session.handleMethod`, which drops
// `MethodResult.Response` whenever `Err` is set. A test that calls
// `tlsMethod.Process` directly cannot see that layer at all, so the three tests
// beside this one prove the split by composition and this one proves it end to
// end. MEASURED: with the pre-fix `{Response: alert, Err: failure}` shape
// restored, the direct tests redden and `TestEAPTLSSessionSendsFailureForRejected
// ClientCertificate` stays GREEN, because an EAP-Failure is sent either way.
//
// RFC requirement: RFC5216-2.1.3-4 positive -- RFC 5216 Section 2.1.3: "the
// EAP-Server MUST send an EAP-Failure packet and terminate the conversation."
// This is the packet-level proof: an EAP-Failure reaches the peer, after the
// alert. tlsMethod.Process alone can only show a MethodResult.Err, which is not
// yet a packet.
//
// VALIDATES: the peer receives an EAP-Request carrying a TLS record, and the
// EAP-Failure arrives only on a LATER packet.
// PREVENTS: the discarded-Response defect returning behind a green session test.
func TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure(t *testing.T) {
	impostor := newImpostorPKI(t)

	sess, err := NewSession(TypeTLS, impostor.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	peer := NewPeerSessionTLS("impostor-pki-client", impostor.peerConfig())
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	// Every packet the AUTHENTICATOR sent, in order, as the peer saw it.
	var sent []*Packet
	req := sess.Begin()
	for range 40 {
		sent = append(sent, req)
		pres := peer.Process(req)
		if pres.Response == nil {
			break
		}
		next := sess.Process(pres.Response)
		if next == nil {
			break
		}
		req = next
	}
	sent = append(sent, req)

	failureAt := -1
	alertAt := -1
	for i, p := range sent {
		if p.Code == CodeFailure && failureAt < 0 {
			failureAt = i
		}
		// A TLS record, not the bare fragment ACK a quiet round produces. The
		// record opens behind the EAP-TLS header, whose size RFC 9190 Section
		// 2.1.9 makes one octet on an unfragmented message and five on a
		// fragmented one.
		body := tlsBytesFromTypeData(p.TypeData)
		if p.Code == CodeRequest && p.Type == TypeTLS && len(body) > 0 &&
			alertAt < 0 && failureAt < 0 {
			ct := body[0]
			if ct == tlsRecordAlert || ct == tlsRecordApplicationData {
				alertAt = i
			}
		}
	}

	if failureAt < 0 {
		t.Fatalf("the authenticator never sent EAP-Failure across %d packets", len(sent))
	}
	if alertAt < 0 {
		t.Fatalf("no EAP-Request carrying a TLS record reached the peer before EAP-Failure "+
			"(packet %d): Session.handleMethod discarded the alert and the peer learns only "+
			"that the exchange ended, never why", failureAt)
	}
	if alertAt >= failureAt {
		t.Errorf("the alert was packet %d and EAP-Failure packet %d; RFC 5216 Section 2.1.3 "+
			"puts the alert first and waits for the peer's reply", alertAt, failureAt)
	}
	if sess.Succeeded() {
		t.Error("the session reports success for a client certificate it could not verify")
	}
}

// TestEAPTLSRejectedPeerCannotSteerTheReportedCause asserts the recorded cause
// survives whatever the rejected peer answers the alert with.
//
// VALIDATES: after the alert round, a malformed EAP-TLS response and a response
// of a type this method does not serve each leave Session.Err holding the TLS
// handshake failure, and each draws the EAP-Failure the conversation owes.
// PREVENTS: a peer whose certificate was refused choosing what the operator sees.
// The reply reaches stateLastWord (eap.go), which answers it without asking the
// method anything. Were it fed to tlsMethod.Process instead, the first guard that
// tripped would win and the certificate failure would be replaced by "peer ended
// a TLS message after 0 of 10 declared bytes" -- which names the peer's last
// packet instead of the reason the exchange died (ai/rules/cli.md).
func TestEAPTLSRejectedPeerCannotSteerTheReportedCause(t *testing.T) {
	// Two shapes a rejected peer can answer with, each of which named the peer's
	// own packet rather than the refusal before the cause was recorded.
	replies := []struct {
		name    string
		reply   func(identifier uint8) *Packet
		usurper string // the wording that would win if the recorded cause did not
	}{
		{
			// A first fragment declaring ten octets and carrying none. Fed to a live
			// exchange this trips reassemblyComplete and is reported as such.
			name: "a truncated TLS message",
			reply: func(id uint8) *Packet {
				return &Packet{Code: CodeResponse, Identifier: id, Type: TypeTLS, TypeData: []byte{eapTLSFlagL, 0, 0, 0, 10}}
			},
			usurper: "declared bytes",
		},
		{
			// A type check this method does not serve, which answers ErrMethodFailed
			// and names no cause at all.
			name: "an EAP type this method does not serve",
			reply: func(id uint8) *Packet {
				return &Packet{Code: CodeResponse, Identifier: id, Type: TypeIdentity}
			},
			usurper: "method authentication failed",
		},
	}

	for _, tc := range replies {
		t.Run(tc.name, func(t *testing.T) {
			impostor := newImpostorPKI(t)
			sess, err := NewSession(TypeTLS, impostor.serverConfig())
			if err != nil {
				t.Fatalf("NewSession: %v", err)
			}
			peer := NewPeerSessionTLS("impostor-pki-client", impostor.peerConfig())
			t.Cleanup(func() {
				sess.Close()
				peer.Close()
			})

			alert := alertFlight(t, sess, peer, 40)

			last := sess.Process(tc.reply(alert.Identifier))
			if last == nil {
				t.Fatal("a rejected peer's reply drew no packet, so the conversation never terminated")
			}
			if last.Code != CodeFailure {
				t.Errorf("a rejected peer's reply drew code %d, want %d (EAP-Failure)", last.Code, CodeFailure)
			}
			msg := sess.Err().Error()
			if !strings.Contains(msg, "unknown authority") {
				t.Errorf("error %q is not the certificate failure that caused the rejection", msg)
			}
			if strings.Contains(msg, tc.usurper) {
				t.Errorf("error %q reports the peer's reply instead of the rejection that "+
					"preceded it: the rejected peer chose what the operator reads", msg)
			}
		})
	}
}

// TestEAPTLSAuthenticatorReportsTheFailureWithNoAlertToSend asserts the split
// does not strand an exchange whose engine produced no alert.
//
// RFC requirement: RFC5216-2.1.3-3 negative -- the wait RFC 5216 Section 2.1.3
// requires exists "to ensure that the peer receives the TLS alert message", so it
// is owed only when an alert actually went out. With none sent there is nothing
// for the peer to receive and nothing for it to reply to, and the authenticator
// concludes in that same round instead of waiting.
//
// VALIDATES: when the TLS engine records an error but leaves nothing to send,
// Process reports the failure in that same round rather than waiting for a reply
// to a packet it never sent.
// PREVENTS: the parked-error fix turning into a hang. The peer has nothing to
// answer, so an authenticator that waited would answer bare ACKs until the
// stale-handshake reaper -- the regression the handshake-error read removed.
func TestEAPTLSAuthenticatorReportsTheFailureWithNoAlertToSend(t *testing.T) {
	pki := newEAPTLSPKI(t)
	method, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}

	// The engine failed and wrote nothing: err set, no serverBuf, finished.
	tr := newEAPTLSTransport()
	tr.setError(errNoPeerTrustAnchor)
	method.transport = tr
	method.started.Store(true)
	method.state = tlsStateHandshake
	t.Cleanup(method.Close)

	res := method.Process(&Packet{Code: CodeResponse, Type: TypeTLS, TypeData: []byte{0}})
	if res.Err == nil {
		t.Fatal("a failed handshake with no alert to send reported nothing: the peer would be " +
			"ACKed until the reaper fires")
	}
	if res.Response != nil {
		t.Errorf("the authenticator sent %+v when its engine had produced no alert", res.Response)
	}
}
