// Design: docs/guide/l2tp.md -- proxied PAP renegotiates LCP when the peer sends no request.

package ppp

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// TestProxiedPAPWithoutRequestRenegotiatesLCP plays the LAC that finished PAP
// before ICCN: its peer is in the Network phase and never sends another
// Authenticate-Request. Ze MUST leave the proxied lifetime rather than fail the
// session at papFirstRequestTimeout, and the fresh negotiation MUST complete
// through PAP and IPCP to a usable session. Method: the real Driver, with no PAP
// frame on the wire until Ze's Configure-Request has restarted LCP.
func TestProxiedPAPWithoutRequestRenegotiatesLCP(t *testing.T) {
	td := startProxyTransmitCapDriver(t, 1400)
	assertProxyTransmitNegotiation(t, td.driver, 1400)

	ev := waitForEvent(t, td.driver.EventsOut(), proxyPAPRequestWait+2*time.Second)
	down, ok := ev.(EventLCPDown)
	if !ok || down.NetworkPhase {
		t.Fatalf("event = %+v, want LCPDown from the authentication phase", ev)
	}
	select {
	case authEvent := <-td.driver.AuthEventsOut():
		t.Fatalf("no credential was presented, got %+v", authEvent)
	default:
	}

	request := readLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureRequest)
	opts, err := ParseLCPOptions(request.Data)
	if err != nil {
		t.Fatal(err)
	}
	if mru, present := lookupMRUOption(opts); !present || mru != 1492 {
		t.Fatalf("replacement local MRU = %d, present=%t; want 1492", mru, present)
	}
	auth, present := lookupOption(opts, LCPOptAuthProto)
	if !present || !bytes.Equal(auth, []byte{0xc0, 0x23}) {
		t.Fatalf("replacement Auth-Protocol = %x, want PAP", auth)
	}

	writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureRequest, 0x51, nil)
	readLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck)
	writeLifecyclePacket(t, td.peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
	assertProxyTransmitNegotiation(t, td.driver, 1500)

	completeProxyTransmitNetwork(t, td)
	assertProxyTransmitInterface(t, td, 1492, 1)
}

// TestProxiedLCPRenegotiationEndsTheProxiedLifetime covers what the Driver test
// cannot reach. The replacement lifetime waits the full first-request bound again
// because it is no longer proxied, and a Configure-Request that cannot be sent
// leaves LCP in Opened, which is the state the callers read as the session ending.
func TestProxiedLCPRenegotiationEndsTheProxiedLifetime(t *testing.T) {
	s, rec, events := newRFC1661Session(LCPStateOpened)
	s.lcpProxied = true
	s.renegotiateProxiedLCP()
	if s.lcpProxied {
		t.Fatal("the replacement lifetime still reads as proxied")
	}
	if state := s.currentState(); state != LCPStateReqSent {
		t.Fatalf("state = %s, want req-sent", state)
	}
	if _, found := findCode(t, rec, LCPConfigureRequest); !found {
		t.Fatal("renegotiation sent no Configure-Request")
	}
	if down, ok := (<-events).(EventLCPDown); !ok || down.NetworkPhase {
		t.Fatal("renegotiation did not report LCP down from the authentication phase")
	}

	failed, _, failedEvents := newRFC1661Session(LCPStateOpened)
	failed.lcpProxied = true
	failed.chanFile = &failingPAPReplyWriter{err: io.ErrClosedPipe}
	failed.renegotiateProxiedLCP()
	if state := failed.currentState(); state != LCPStateOpened {
		t.Fatalf("unsent renegotiation committed state %s, want opened", state)
	}
	// renegotiateProxiedLCP runs on this goroutine, so its events are queued now,
	// and a no-op fails on the count instead of hanging on the receive.
	if queued := len(failedEvents); queued != 2 {
		t.Fatalf("unsent renegotiation queued %d events, want LCPDown and SessionDown", queued)
	}
	if down, ok := (<-failedEvents).(EventLCPDown); !ok || down.NetworkPhase {
		t.Fatal("unsent renegotiation did not report LCP down from the authentication phase")
	}
	if _, ok := (<-failedEvents).(EventSessionDown); !ok {
		t.Fatal("unsent renegotiation did not report the session down")
	}
}

// TestProxiedPAPWaitIsShorterThanTheFirstRequestBound holds the two bounds apart:
// a proxied wait equal to the ordinary one would never renegotiate in time to be
// worth having, and a wait below two Restart periods would give up on a peer that
// is still repeating its Authenticate-Request.
func TestProxiedPAPWaitIsShorterThanTheFirstRequestBound(t *testing.T) {
	if proxyPAPRequestWait >= papFirstRequestTimeout {
		t.Fatalf("proxied wait %s is not below the first-request bound %s", proxyPAPRequestWait, papFirstRequestTimeout)
	}
	if proxyPAPRequestWait < 2*defaultRestartTimer {
		t.Fatalf("proxied wait %s is below two Restart periods", proxyPAPRequestWait)
	}
}
