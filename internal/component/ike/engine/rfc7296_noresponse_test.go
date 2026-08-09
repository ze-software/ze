// RFC 7296 Section 3.1: an IKE endpoint never answers a message that already carries
// the Response flag. Four producers enforce it, one per path a message can arrive on.
//
// Helpers here start with `nrs`, so they cannot collide with the sibling RFC files in
// this package. This file reuses the `rtx` loopback helpers and the `rky` recorder.

// rfc-test-change-approved: 2026-07-31 the owner gave standing approval, for the whole of
// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, to strengthen tagged tests. `net` is imported so the
// "out of SA admission" arm can give its packet a source address and actually reach the
// producer its tag names.

package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// nrsFlip returns a copy of raw with the Response flag set or cleared. Only octet 19
// changes, so the message is otherwise the one the engine built.
func nrsFlip(raw []byte, response bool) []byte {
	out := append([]byte(nil), raw...)
	if response {
		out[19] |= wire.FlagResponse
	} else {
		out[19] &^= wire.FlagResponse
	}
	return out
}

// nrsInformational builds an INFORMATIONAL message at msgID with the given flags.
func nrsInformational(t *testing.T, sa *SA, msgID uint32, flags uint8) []byte {
	t.Helper()
	raw, err := buildEncryptedMessageEx(sa, nil, msgID, wire.ExchangeInformational, flags)
	if err != nil {
		t.Fatalf("build an INFORMATIONAL message at id %d: %v", msgID, err)
	}
	return raw
}

// VALIDATES: every path that receives a message marked as a response declines to build
// an answer for it, and the matching request on the same path IS answered.
// PREVENTS: two Ze instances echoing one message forever, because a response carries
// the Message ID of the request it answers.
//
// RFC requirement: RFC7296-3.1-12 positive -- RFC 7296 Section 3.1 forbids an endpoint to
// generate a response to a message that is already marked as a response. It names one
// exception, in Section 2.21.2, which WP-3 owns. The checklist row carries the sentence
// verbatim (rfc/full/rfc7296.txt:4122-4127).
//
// Four producers enforce it. handleResponderInbound drops a flagged message first
// (responder.go:53-56). tryResponderSAInit refuses to admit one (register.go:588).
// handleInformationalOwned returns before its response builder (inbound.go).
// handleCreateChildSAOwned's isResponse branch has no send path (inbound.go).
//
// RFC requirement: RFC7296-3.1-12 negative -- each path DOES answer the same message with the
// flag cleared. The silence is therefore the R bit speaking, and not a handler that
// writes nothing.
func TestNrsResponseNeverDrawsAResponse(t *testing.T) {
	log := slogutil.DiscardLogger()

	// Producer one: the responder handshake path.
	t.Run("responder handshake", func(t *testing.T) {
		iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "noresp-psk")
		build := func() (*SA, []byte) {
			ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
			if err != nil {
				t.Fatalf("newInitiatorSA: %v", err)
			}
			return ini, buildSAInitRequest(ini, testIKEGroup())
		}
		run := func(raw []byte, iniSPI [8]byte) *SA {
			resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), iniSPI)
			if err != nil {
				t.Fatalf("newResponderSA: %v", err)
			}
			ps := &PeerSession{peerName: "ze", peerCfg: respPeer,
				ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			ps.handleResponderInbound(resp, parseMsg(t, raw), transport.Packet{Data: raw}, nil, log)
			return resp
		}

		ini, req := build()
		marked := run(nrsFlip(req, true), ini.InitiatorSPI)
		if marked.State != StateIdle {
			t.Errorf("responder state = %v for a message marked as a response, want idle",
				marked.State)
		}
		if len(marked.LastSentMsg) != 0 {
			t.Error("the responder built an answer to a message marked as a response")
		}

		// Negative. The same message with the flag clear IS answered.
		ini2, req2 := build()
		plain := run(nrsFlip(req2, false), ini2.InitiatorSPI)
		if plain.State != StateSAInitReceived {
			t.Errorf("responder state = %v for a request, want sa-init-received", plain.State)
		}
		if len(plain.LastSentMsg) == 0 {
			t.Error("the responder built no answer to a request, so the silence above " +
				"proves nothing")
		}
	})

	// Producer two: out-of-SA IKE_SA_INIT admission.
	//
	// rfc-test-change-approved: 2026-07-31 the owner gave standing approval, for the whole
	// of docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, to strengthen a tagged test whose body did
	// not reach the producer it named. This arm sent a packet with no RemoteAddr. So
	// matchResponderPeer(nil) returned nil (register.go:557-559), and register.go:594-597
	// returned false whatever the R-flag check did. Deleting that check left the arm green,
	// and its negative control asserted nothing in either branch. The approval covers
	// strengthening only, never weakening.
	t.Run("out of SA admission", func(t *testing.T) {
		iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "noresp-admit")
		ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
		if err != nil {
			t.Fatalf("newInitiatorSA: %v", err)
		}
		req := buildSAInitRequest(ini, testIKEGroup())

		// The admission path has to be REACHABLE for the refusal below to mean anything.
		// tryResponderSAInit declines a source that matches no configured peer before it
		// reads the flag. So a `respond` peer is registered here, and the packet is given
		// the source address that peer names. The flag is then the ONLY difference between
		// the two calls.
		ps := &PeerSession{peerName: "noresp-admit", peerCfg: respPeer,
			ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
		setActivePeers(map[string]*PeerSession{ps.peerName: ps})
		t.Cleanup(func() { setActivePeers(nil) })
		src := &net.UDPAddr{IP: net.ParseIP(respPeer.RemoteAddress), Port: 500}
		if matchResponderPeer(src) != ps {
			t.Fatal("the registered responder peer does not match the packet source; the " +
				"admission path is unreachable and neither call below tests the R bit")
		}

		var zeroSPI [8]byte
		if tryResponderSAInit(transport.Packet{Data: nrsFlip(req, true), RemoteAddr: src},
			ini.InitiatorSPI, zeroSPI, NewSATable(), nil, log) {
			t.Error("a message marked as a response created a responder SA; it must never " +
				"be admitted, because admitting it draws an answer")
		}
		if ps.responderBusy.Load() {
			t.Error("a message marked as a response reached the half-open admission gate; " +
				"the R-bit check must refuse it before any state is taken")
		}

		// Negative: the SAME packet with the flag cleared IS admitted. The refusal above is
		// therefore the Response flag speaking, and not a source the responder never
		// matched.
		if !tryResponderSAInit(transport.Packet{Data: nrsFlip(req, false), RemoteAddr: src},
			ini.InitiatorSPI, zeroSPI, NewSATable(), nil, log) {
			t.Error("an IKE_SA_INIT request from a configured responder peer was not " +
				"admitted, so the refusal above proves nothing about the R bit")
		}
	})

	// Producer three: the established INFORMATIONAL path.
	t.Run("established informational", func(t *testing.T) {
		peer, sa, ps := establishPSK(t)
		peerTr, myTr := rtxPeerLink(t)
		sa.PeerCfg.RemoteAddress = "127.0.0.1"
		remote := sa.remoteUDPAddr()
		if remote == nil {
			t.Fatal("the SA has no resolvable peer address")
		}

		// A message that is already a response draws nothing, even at the id our own
		// ExpectedMsgID names.
		asResponse := nrsInformational(t, peer, sa.ExpectedMsgID,
			initiatorFlag(peer)|wire.FlagResponse)
		ps.handleOwnedInbound(sa, transport.Packet{Data: asResponse}, myTr, nil, log)
		rtxExpectSilence(t, peerTr, myTr, remote, "an INFORMATIONAL marked as a response")

		// Negative. The same exchange as a REQUEST is answered.
		asRequest := nrsInformational(t, peer, sa.ExpectedMsgID, initiatorFlag(peer))
		ps.handleOwnedInbound(sa, transport.Packet{Data: asRequest}, myTr, nil, log)
		answer := rtxRecv(t, peerTr)
		if answer == nil {
			t.Fatal("an INFORMATIONAL request drew no answer, so the silence above proves nothing")
		}
		if parseMsg(t, answer).Header.Flags&wire.FlagResponse == 0 {
			t.Error("the answer to a request does not carry the Response flag")
		}
	})

	// Producer four: the established CREATE_CHILD_SA path.
	t.Run("established create child sa", func(t *testing.T) {
		peer, sa, ps := establishPSK(t)
		peerTr, myTr := rtxPeerLink(t)
		sa.PeerCfg.RemoteAddress = "127.0.0.1"
		remote := sa.remoteUDPAddr()
		if remote == nil {
			t.Fatal("the SA has no resolvable peer address")
		}
		dp := &rkyDP{}
		old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		ps.setChildSA(old)

		// A CREATE_CHILD_SA marked as a response, with no exchange of ours outstanding,
		// writes nothing.
		req, _, err := initiateChildRekey(peer, old)
		if err != nil {
			t.Fatalf("initiateChildRekey: %v", err)
		}
		inner, err := decryptAndParse(sa, parseMsg(t, req), req)
		if err != nil {
			t.Fatalf("decrypt the peer rekey request: %v", err)
		}
		respMsg := parseMsg(t, nrsFlip(req, true))
		ps.handleCreateChildSAOwned(sa, respMsg, inner, true, myTr, dp, log)
		rtxExpectSilence(t, peerTr, myTr, remote, "a CREATE_CHILD_SA marked as a response")

		// Negative. The same payload chain as a REQUEST is answered.
		ps.handleCreateChildSAOwned(sa, parseMsg(t, req), inner, false, myTr, dp, log)
		if rtxRecv(t, peerTr) == nil {
			t.Fatal("a CREATE_CHILD_SA rekey request drew no answer, so the silence above " +
				"proves nothing")
		}
	})
}

// TestNrsBuiltResponsesEchoTheRequestID records why answering a response would loop.
// A response carries the Message ID of the request it answers. An endpoint that answered
// one would therefore send a message the peer classifies exactly as the first.
func TestNrsBuiltResponsesEchoTheRequestID(t *testing.T) {
	log := slogutil.DiscardLogger()
	peer, sa, ps := establishPSK(t)
	const reqID uint32 = 9
	sa.ExpectedMsgID = reqID

	req := nrsInformational(t, peer, reqID, initiatorFlag(peer))
	ps.handleOwnedInbound(sa, transport.Packet{Data: req}, nil, nil, log)
	if !sa.lastResponseSet {
		t.Fatal("no response was cached for the request")
	}
	if sa.lastResponseID != reqID {
		t.Errorf("the cached response answers id %d, want %d", sa.lastResponseID, reqID)
	}
	if got := parseMsg(t, sa.lastResponse).Header.MessageID; got != reqID {
		t.Errorf("the response carries Message ID %d, want %d; an answer to an answer "+
			"would therefore be indistinguishable from the original", got, reqID)
	}
}

// VALIDATES: handleInformationalOwned itself refuses to build an answer when the message
// it processes is a response, and builds one when it is a request.
// PREVENTS: the guard being reachable only through classifyInbound. An INFORMATIONAL
// response that DOES match an outstanding exchange reaches this handler directly, so the
// handler must carry the rule and not only its caller.
//
// RFC requirement: RFC7296-3.1-12 positive -- the third producer, taken at its own entry point.
// handleInformationalOwned (inbound.go) returns before its response builder when the
// message is a response. RFC 7296 Section 3.1's MUST NOT therefore holds on the path
// where a matching outstanding exchange carries the message past classifyInbound.
//
// RFC requirement: RFC7296-3.1-12 negative -- the same handler answers the same message when it
// is a request, and caches that answer. The refusal is the R bit and not a dead builder.
func TestNrsInformationalHandlerRefusesAResponse(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, sa, ps := establishPSK(t)
	peerTr, myTr := rtxPeerLink(t)
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	remote := sa.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the SA has no resolvable peer address")
	}

	msg := &wire.Message{Header: wire.Header{
		MessageID:    sa.ExpectedMsgID,
		ExchangeType: wire.ExchangeInformational,
	}}

	// Marked as a response: nothing is written and nothing is cached.
	sa.lastResponse = nil
	sa.lastResponseSet = false
	ps.handleInformationalOwned(sa, msg, nil, true, myTr, nil, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "handleInformationalOwned given a response")
	if sa.lastResponseSet {
		t.Error("a response was cached for a message that is itself a response")
	}

	// Negative: the same message as a request IS answered.
	ps.handleInformationalOwned(sa, msg, nil, false, myTr, nil, log)
	answer := rtxRecv(t, peerTr)
	if answer == nil {
		t.Fatal("handleInformationalOwned wrote nothing for a request, so the silence " +
			"above proves nothing")
	}
	if parseMsg(t, answer).Header.Flags&wire.FlagResponse == 0 {
		t.Error("the answer to a request does not carry the Response flag")
	}
	if !sa.lastResponseSet {
		t.Error("the answer to a request was not cached")
	}
}
