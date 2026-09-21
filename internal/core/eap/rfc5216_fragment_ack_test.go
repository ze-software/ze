// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS fragmentation
// RFC: rfc/short/rfc5216.md -- Section 2.1.5, the fragment ACK and its Identifier
//
// RFC 5216 Section 2.1.5 runs a fragmented message as a lockstep of fragment and
// ACK in each direction:
//
//	"When an EAP-TLS peer receives an EAP-Request packet with the M bit set, it
//	MUST respond with an EAP-Response with EAP-Type=EAP-TLS and no data. This
//	serves as a fragment ACK. The EAP server MUST wait until it receives the
//	EAP-Response before sending another fragment. In order to prevent errors in
//	processing of fragments, the EAP server MUST increment the Identifier field
//	for each fragment contained within an EAP-Request, and the peer MUST include
//	this Identifier value in the fragment ACK contained within the EAP-Response."
//
// and the same three sentences again with the roles swapped for a fragmented
// EAP-Response.
//
// The existing fragmentation tests (rfc9190_fragmentation_test.go) drive the
// fragmenter alone. This file drives the real Session against the real
// PeerSession with certificates large enough that BOTH directions fragment, and
// reads the lockstep off the wire: every fragment, every ACK, every Identifier.
// At each fragment it also probes the side that is waiting, to show that
// nothing goes out until the packet the RFC names arrives.
//
// VALIDATES: the six MUSTs of the two paragraphs, on a conversation that ends
// in EAP-Success with one MSK on both sides, so the fragments are the real
// handshake and not a shape the test invented.
// PREVENTS: an ACK that carries data, a fragment that goes out before its ACK,
// an Identifier that stalls across fragments, and an ACK or a fragment that
// answers a stale Identifier and is still accepted.
package eap

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"testing"
	"time"
)

// fragmentedFlight is one conversation, recorded packet by packet, with the
// probe outcomes gathered while it ran. serverSent[i] is answered by
// peerSent[i].
type fragmentedFlight struct {
	serverSent []*Packet
	peerSent   []*Packet
	successAt  int
	serverMSK  [64]byte
	peerMSK    [64]byte
	peerDone   bool

	// serverFragments counts the Requests that carried M, and peerFragments the
	// Responses that did. A test refuses a conversation in which either stayed
	// zero, because such a conversation shows nothing about fragmentation.
	serverFragments int
	peerFragments   int
}

// isFragment reports whether an EAP-TLS packet carries the M bit.
func isFragment(p *Packet) bool {
	return p != nil && p.Type == TypeTLS && len(p.TypeData) > 0 && p.TypeData[0]&eapTLSFlagM != 0
}

// newFragmentingPKI issues a server and a client certificate whose subject is
// long enough that each side's certificate flight spans several EAP-TLS
// fragments. Both chain to the harness CA, so the handshake succeeds.
func newFragmentingPKI(t *testing.T) (MethodConfig, *PeerSession) {
	t.Helper()
	pki := newEAPTLSPKI(t)
	long := strings.Repeat("x", 2*eapTLSFragmentSize)
	serverCert, serverKey := newLeaf(t, pki.trustedCA, pki.trustedCAKey, "server-"+long, 701, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCert, clientKey := newLeaf(t, pki.trustedCA, pki.trustedCAKey, "client-"+long, 702, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	cfg := MethodConfig{
		ServerCertPEM: serverCert,
		ServerKeyPEM:  serverKey,
		CACertPEM:     pki.trustedCAPEM,
		CRLPEM:        pki.trustedCRLPEM,
		Resumption:    NewResumption(time.Now, true),
	}
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   clientCert,
		KeyPEM:    clientKey,
		CACertPEM: pki.trustedCAPEM,
		CRLPEM:    pki.trustedCRLPEM,
	})
	return cfg, peer
}

// driveFragmentedFlight runs the conversation over TLS 1.2 and, at every
// fragment, probes the side that owes the next one.
//
// After the authenticator sends a fragment, it is handed a Response carrying an
// Identifier that answers no outstanding Request. After the peer sends one, the
// peer is handed a packet with an undefined Code, which RFC 3748 Section 4 makes
// it discard, and the authenticator is handed the peer's fragment re-numbered
// with a stale Identifier. Each probe MUST draw nothing and MUST leave the
// fragmenter where it was; the test fails on the spot otherwise.
func driveFragmentedFlight(t *testing.T, cfg MethodConfig, peer *PeerSession) *fragmentedFlight {
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
	method.tlsConfig.MaxVersion = tls.VersionTLS12

	fl := &fragmentedFlight{successAt: -1}
	req := sess.Begin()
	for range 60 {
		fl.serverSent = append(fl.serverSent, req)
		if req.Code == CodeSuccess {
			fl.successAt = len(fl.serverSent) - 1
			fl.serverMSK = sess.MSK()
		}
		if isFragment(req) {
			fl.serverFragments++
			// RFC5216-2.1.5-3 and -4: no further fragment before the ACK, and an
			// ACK with a stale Identifier is not that ACK.
			before := method.outOffset
			stale := sess.Process(&Packet{Code: CodeResponse, Identifier: req.Identifier - 1, Type: TypeTLS, TypeData: []byte{0}})
			if stale != nil {
				t.Fatalf("after fragment %d (Identifier %d) the authenticator answered an ACK carrying Identifier %d with code %d flags %x: it sent another fragment before the ACK for this one arrived",
					len(fl.serverSent)-1, req.Identifier, req.Identifier-1, stale.Code, stale.TypeData[:1])
			}
			if method.outOffset != before {
				t.Fatalf("after fragment %d the stale ACK moved the authenticator's fragment offset from %d to %d", len(fl.serverSent)-1, before, method.outOffset)
			}
		}

		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("the peer failed at packet %d: %v", len(fl.serverSent)-1, pres.Err)
		}
		if pres.Response != nil {
			fl.peerSent = append(fl.peerSent, pres.Response)
		}
		if pres.Done {
			fl.peerDone = true
			fl.peerMSK = pres.MSK
			break
		}
		if pres.Response == nil {
			break
		}
		if isFragment(pres.Response) {
			fl.peerFragments++
			// RFC5216-2.1.5-6: the peer sends no further fragment until an
			// EAP-Request arrives. A packet it discards is not one.
			before := peer.outOffset
			if probe := peer.Process(&Packet{Code: 0, Identifier: req.Identifier, Type: TypeTLS, TypeData: []byte{0}}); probe.Response != nil || probe.Err != nil {
				t.Fatalf("after its fragment %d the peer answered a discarded packet with response=%+v err=%v", len(fl.peerSent)-1, probe.Response, probe.Err)
			}
			if peer.outOffset != before {
				t.Fatalf("after its fragment %d a discarded packet moved the peer's fragment offset from %d to %d", len(fl.peerSent)-1, before, peer.outOffset)
			}
			// RFC5216-2.1.5-7: the subsequent fragment carries the ACK's
			// Identifier, and one that does not is discarded.
			renumbered := *pres.Response
			renumbered.Identifier = req.Identifier - 1
			if stale := sess.Process(&renumbered); stale != nil {
				t.Fatalf("the authenticator accepted the peer's fragment %d re-numbered with Identifier %d (outstanding %d) and answered code %d flags %x",
					len(fl.peerSent)-1, renumbered.Identifier, req.Identifier, stale.Code, stale.TypeData[:1])
			}
		}

		next := sess.Process(pres.Response)
		if next == nil {
			break
		}
		req = next
	}

	if fl.successAt < 0 || !fl.peerDone {
		t.Fatalf("the conversation did not end in EAP-Success on both sides (successAt=%d peerDone=%v, %d server packets)", fl.successAt, fl.peerDone, len(fl.serverSent))
	}
	var zero [64]byte
	if fl.serverMSK == zero || fl.serverMSK != fl.peerMSK {
		t.Fatalf("the two sides do not share a non-zero MSK (server zero=%v equal=%v): the fragments did not carry a real handshake",
			fl.serverMSK == zero, fl.serverMSK == fl.peerMSK)
	}
	if fl.serverFragments == 0 || fl.peerFragments == 0 {
		t.Fatalf("the conversation carried %d authenticator fragments and %d peer fragments; both must fragment for this file to show anything",
			fl.serverFragments, fl.peerFragments)
	}
	return fl
}

// TestRFC5216PeerAcksEachServerFragmentWithNoData reads what the peer answers
// to each Request carrying M, and what it answers to the last fragment.
//
// RFC requirement: RFC5216-2.1.5-2 positive -- RFC 5216 Section 2.1.5: "When an
// EAP-TLS peer receives an EAP-Request packet with the M bit set, it MUST
// respond with an EAP-Response with EAP-Type=EAP-TLS and no data." Every
// Request carrying M is answered by an EAP-TLS Response whose TypeData is the
// single flags octet 0x00.
//
// RFC requirement: RFC5216-2.1.5-2 negative -- the no-data Response answers a
// fragment with M set and nothing else. The last fragment of the authenticator's
// certificate flight, which clears M, is answered with TLS data: the peer's own
// certificate flight.
func TestRFC5216PeerAcksEachServerFragmentWithNoData(t *testing.T) {
	cfg, peer := newFragmentingPKI(t)
	fl := driveFragmentedFlight(t, cfg, peer)

	lastFragmentAnswered := false
	for i, req := range fl.serverSent {
		if i >= len(fl.peerSent) {
			break
		}
		res := fl.peerSent[i]
		if isFragment(req) {
			if !bareEAPTLSResponse(res) {
				t.Errorf("Request %d carries M and was answered with code %d type %d TypeData %x, want an EAP-TLS Response with no data",
					i, res.Code, res.Type, res.TypeData)
			}
			continue
		}
		if i > 0 && isFragment(fl.serverSent[i-1]) && !lastFragmentAnswered {
			lastFragmentAnswered = true
			if bareEAPTLSResponse(res) {
				t.Errorf("Request %d is the last fragment (M clear) and was answered with the no-data ACK; the peer owed its TLS flight", i)
			}
		}
	}
	if !lastFragmentAnswered {
		t.Fatal("no last fragment was found in the conversation")
	}
}

// TestRFC5216ServerSendsTheNextFragmentOnlyInAnswerToTheAck reads the packet
// after each authenticator fragment.
//
// RFC requirement: RFC5216-2.1.5-3 positive -- RFC 5216 Section 2.1.5: "The EAP
// server MUST wait until it receives the EAP-Response before sending another
// fragment." The fragment after each Request carrying M is the Request that
// answers the peer's ACK for it, and it continues the same message (no S bit,
// and no length field, which only the first fragment carries).
//
// RFC requirement: RFC5216-2.1.5-3 negative -- driveFragmentedFlight hands the
// authenticator, after each fragment, a Response that answers no outstanding
// Request. It draws nothing and the fragment offset does not move: no fragment
// goes out before the EAP-Response for the previous one.
func TestRFC5216ServerSendsTheNextFragmentOnlyInAnswerToTheAck(t *testing.T) {
	cfg, peer := newFragmentingPKI(t)
	fl := driveFragmentedFlight(t, cfg, peer)

	for i, req := range fl.serverSent {
		if !isFragment(req) || i+1 >= len(fl.serverSent) {
			continue
		}
		next := fl.serverSent[i+1]
		if next.Code != CodeRequest || next.Type != TypeTLS {
			t.Fatalf("after fragment %d the authenticator sent code %d type %d, want the next fragment", i, next.Code, next.Type)
		}
		if next.TypeData[0]&eapTLSFlagS != 0 || next.TypeData[0]&eapTLSFlagL != 0 || len(next.TypeData) < 2 {
			t.Errorf("the packet after fragment %d has flags %02x and %d octets; want a continuation fragment carrying TLS data with neither S nor L",
				i, next.TypeData[0], len(next.TypeData))
		}
	}
}

// TestRFC5216ServerFragmentIdentifiersIncrementAndTheAckEchoesThem reads the
// Identifier of each authenticator fragment and of the ACK that answers it.
//
// RFC requirement: RFC5216-2.1.5-4 positive -- RFC 5216 Section 2.1.5: "the EAP
// server MUST increment the Identifier field for each fragment contained within
// an EAP-Request, and the peer MUST include this Identifier value in the
// fragment ACK contained within the EAP-Response." Each fragment's Identifier is
// the previous Request's plus one, and each ACK carries its fragment's.
//
// RFC requirement: RFC5216-2.1.5-4 negative -- driveFragmentedFlight hands the
// authenticator, after each fragment, an ACK carrying the previous Identifier.
// It is discarded: an ACK that does not include the fragment's Identifier is not
// the ACK.
func TestRFC5216ServerFragmentIdentifiersIncrementAndTheAckEchoesThem(t *testing.T) {
	cfg, peer := newFragmentingPKI(t)
	fl := driveFragmentedFlight(t, cfg, peer)

	for i, req := range fl.serverSent {
		if !isFragment(req) || i == 0 {
			continue
		}
		if want := fl.serverSent[i-1].Identifier + 1; req.Identifier != want {
			t.Errorf("fragment %d carries Identifier %d, want %d (the previous Request's plus one)", i, req.Identifier, want)
		}
		if i >= len(fl.peerSent) {
			t.Fatalf("fragment %d was never answered", i)
		}
		if ack := fl.peerSent[i]; ack.Identifier != req.Identifier {
			t.Errorf("the ACK for fragment %d carries Identifier %d, want %d", i, ack.Identifier, req.Identifier)
		}
	}
}

// TestRFC5216ServerAcksEachPeerFragmentWithNoData reads what the authenticator
// answers to each Response carrying M, and to the last one.
//
// RFC requirement: RFC5216-2.1.5-5 positive -- RFC 5216 Section 2.1.5: "when
// the EAP server receives an EAP-Response with the M bit set, it MUST respond
// with an EAP-Request with EAP-Type=EAP-TLS and no data." Every Response
// carrying M is answered by an EAP-TLS Request whose TypeData is the single
// flags octet 0x00.
//
// RFC requirement: RFC5216-2.1.5-5 negative -- the no-data Request answers a
// fragment with M set and nothing else. The peer's last fragment, which clears
// M, is answered with TLS data: the authenticator's change_cipher_spec and
// Finished.
func TestRFC5216ServerAcksEachPeerFragmentWithNoData(t *testing.T) {
	cfg, peer := newFragmentingPKI(t)
	fl := driveFragmentedFlight(t, cfg, peer)

	lastFragmentAnswered := false
	for i, res := range fl.peerSent {
		if i+1 >= len(fl.serverSent) {
			break
		}
		next := fl.serverSent[i+1]
		if isFragment(res) {
			if next.Code != CodeRequest || next.Type != TypeTLS || len(next.TypeData) != 1 || next.TypeData[0] != 0 {
				t.Errorf("Response %d carries M and was answered with code %d type %d TypeData %x, want an EAP-TLS Request with no data",
					i, next.Code, next.Type, next.TypeData)
			}
			continue
		}
		if i > 0 && isFragment(fl.peerSent[i-1]) && !lastFragmentAnswered {
			lastFragmentAnswered = true
			if next.Code != CodeRequest || next.Type != TypeTLS || len(next.TypeData) <= 1 {
				t.Errorf("Response %d is the peer's last fragment (M clear) and was answered with code %d type %d TypeData %x; the authenticator owed its closing flight",
					i, next.Code, next.Type, next.TypeData)
			}
		}
	}
	if !lastFragmentAnswered {
		t.Fatal("no last peer fragment was found in the conversation")
	}
}

// TestRFC5216PeerSendsTheNextFragmentOnlyInAnswerToTheAck reads the packet
// after each peer fragment.
//
// RFC requirement: RFC5216-2.1.5-6 positive -- RFC 5216 Section 2.1.5: "The EAP
// peer MUST wait until it receives the EAP-Request before sending another
// fragment." The fragment after each Response carrying M is the Response that
// answers the authenticator's ACK for it, and it continues the same message (no
// length field, which only the first fragment carries).
//
// RFC requirement: RFC5216-2.1.5-6 negative -- driveFragmentedFlight hands the
// peer, after each fragment, a packet with an undefined Code. The peer discards
// it, sends nothing, and its fragment offset does not move: no fragment goes out
// before an EAP-Request arrives.
func TestRFC5216PeerSendsTheNextFragmentOnlyInAnswerToTheAck(t *testing.T) {
	cfg, peer := newFragmentingPKI(t)
	fl := driveFragmentedFlight(t, cfg, peer)

	for i, res := range fl.peerSent {
		if !isFragment(res) || i+1 >= len(fl.peerSent) {
			continue
		}
		next := fl.peerSent[i+1]
		if next.Code != CodeResponse || next.Type != TypeTLS {
			t.Fatalf("after its fragment %d the peer sent code %d type %d, want the next fragment", i, next.Code, next.Type)
		}
		if next.TypeData[0]&eapTLSFlagL != 0 || len(next.TypeData) < 2 {
			t.Errorf("the packet after peer fragment %d has flags %02x and %d octets; want a continuation fragment carrying TLS data with no L bit",
				i, next.TypeData[0], len(next.TypeData))
		}
	}
}

// TestRFC5216ServerAckIdentifiersIncrementAndTheNextFragmentEchoesThem reads
// the Identifier of each fragment ACK the authenticator sends and of the peer
// fragment that follows it.
//
// RFC requirement: RFC5216-2.1.5-7 positive -- RFC 5216 Section 2.1.5: "the EAP
// server MUST increment the Identifier value for each fragment ACK contained
// within an EAP-Request, and the peer MUST include this Identifier value in the
// subsequent fragment contained within an EAP-Response." Each ACK's Identifier
// is the previous Request's plus one, and the peer's next fragment carries it.
//
// RFC requirement: RFC5216-2.1.5-7 negative -- driveFragmentedFlight hands the
// authenticator each peer fragment re-numbered with the previous Identifier. It
// is discarded: a fragment that does not include the ACK's Identifier is not the
// subsequent fragment.
func TestRFC5216ServerAckIdentifiersIncrementAndTheNextFragmentEchoesThem(t *testing.T) {
	cfg, peer := newFragmentingPKI(t)
	fl := driveFragmentedFlight(t, cfg, peer)

	for i, res := range fl.peerSent {
		if !isFragment(res) || i+1 >= len(fl.serverSent) {
			continue
		}
		ack := fl.serverSent[i+1]
		if want := fl.serverSent[i].Identifier + 1; ack.Identifier != want {
			t.Errorf("the ACK for peer fragment %d carries Identifier %d, want %d (the previous Request's plus one)", i, ack.Identifier, want)
		}
		if i+1 >= len(fl.peerSent) {
			t.Fatalf("the ACK for peer fragment %d was never answered", i)
		}
		if next := fl.peerSent[i+1]; next.Identifier != ack.Identifier {
			t.Errorf("the fragment after ACK %d carries Identifier %d, want %d", i+1, next.Identifier, ack.Identifier)
		}
	}
}
