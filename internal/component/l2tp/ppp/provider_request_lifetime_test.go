// Design: docs/architecture/l2tp/subscriber-session-model.md -- provider answers across LCP lifetimes

package ppp

import (
	"errors"
	"testing"
	"time"
)

// A real provider retains the first emitted request while the same PPP worker
// completes a new LCP negotiation and issues another request. Its late old answer
// must be refused; the current request must still authorize that new round.
func TestProviderAnswerFromPriorLCPRoundCannotAuthenticateNewRound(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, owner := frameLeaseStart(t, 11)
	start.SubscriberID = "opaque/access-65543/session-11"
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start

	oldRequest := readProviderLifetimeAuthRequest(t, driver.AuthEventsOut())
	lateProvider := func() error {
		return driver.AuthResponse(oldRequest, true, "", nil)
	}
	freshLCP := restartLifecycleLCP(t, peer)
	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck, freshLCP.Identifier, freshLCP.Data)
	currentRequest := readProviderLifetimeAuthRequest(t, driver.AuthEventsOut())
	if currentRequest.TunnelID != oldRequest.TunnelID {
		t.Fatal("LCP restart replaced the transport's tunnel label")
	}
	if currentRequest.SessionID != oldRequest.SessionID {
		t.Fatal("LCP restart replaced the transport's session label")
	}
	if currentRequest.SubscriberID != oldRequest.SubscriberID {
		t.Fatal("LCP restart replaced the canonical subscriber")
	}
	if owner.closes.Load() != 0 {
		t.Fatal("LCP restart replaced the owned PPP admission")
	}
	if oldRequest.Control != currentRequest.Control {
		t.Fatal("a new admission hid the same-worker request boundary")
	}
	if oldRequest.Active() {
		t.Fatal("old LCP request remains answerable")
	}
	if !currentRequest.Active() {
		t.Fatal("current LCP request is not answerable")
	}
	if err := lateProvider(); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("late old provider result = %v, want request-expired", err)
	}
	if !currentRequest.Active() {
		t.Fatal("late old answer consumed the current request")
	}
	if err := driver.AuthResponse(currentRequest, true, "", nil); err != nil {
		t.Fatalf("current provider answer was refused: %v", err)
	}
	awaitLifecycleUp(t, driver.EventsOut())
	// The new peer Configure-Request carried no Magic-Number option.
	// RFC 1661 Sections 5.8/6.4 require zero in this round's peer Echo;
	// the shared proxy helper still uses the old proxied Magic-Number.
	var frame [64]byte
	n := WriteFrame(frame[:], 0, ProtoLCP, nil)
	n += WriteLCPEcho(frame[:], n, LCPEchoRequest, 0x71, 0, nil)
	if err := peer.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write(frame[:n]); err != nil {
		t.Fatal(err)
	}
	proto, payload := readPeerFrame(t, peer)
	packet, err := ParseLCPPacket(payload)
	if err != nil {
		t.Fatal(err)
	}
	if proto != ProtoLCP || packet.Code != LCPEchoReply || packet.Identifier != 0x71 {
		t.Fatalf("surviving peer reply protocol=%x packet=%+v", proto, packet)
	}
}

func readProviderLifetimeAuthRequest(t *testing.T, events <-chan AuthEvent) EventAuthRequest {
	t.Helper()
	select {
	case event := <-events:
		request, ok := event.(EventAuthRequest)
		if !ok {
			t.Fatalf("provider event = %T, want an authentication request", event)
		}
		return request
	case <-time.After(3 * time.Second):
		t.Fatal("PPP did not emit the provider request")
		return EventAuthRequest{}
	}
}
