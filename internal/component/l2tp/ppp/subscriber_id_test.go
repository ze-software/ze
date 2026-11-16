// Design: docs/research/l2tpv2-ze-integration.md -- opaque subscriber identity at the PPP boundary
// Related: auth.go, ncp.go -- central request emission
// Related: manager.go -- admitted session identity

package ppp

import (
	"testing"
	"time"
)

// TestAuthRequestSubscriberID observes each authentication method's central
// emitter, including an omitted protocol-only identity, without a transport import.
func TestAuthRequestSubscriberID(t *testing.T) {
	for _, id := range []string{"opaque/owner-65543", ""} {
		for _, method := range authMethods {
			t.Run(id+"/"+method.String(), func(t *testing.T) {
				events := make(chan AuthEvent, 1)
				decisions := make(chan authResponseMsg, 1)
				decisions <- authResponseMsg{accept: true}
				session := &pppSession{
					subscriberID:  id,
					authEventsOut: events,
					authRespCh:    decisions,
				}
				request := EventAuthRequest{SubscriberID: "stale-request-owner", Method: method}
				decision, ok := session.awaitAuthDecision(request, "")
				if !ok {
					t.Fatal("authentication decision did not complete")
				}
				if !decision.accept {
					t.Fatal("authentication decision was not preserved")
				}
				request, ok = (<-events).(EventAuthRequest)
				if !ok {
					t.Fatal("emitter did not publish an authentication request")
				}
				if request.SubscriberID != id {
					t.Fatalf("authentication subscriber ID = %q, want %q", request.SubscriberID, id)
				}
			})
		}
	}
}

// TestIPRequestSubscriberID observes both NCP families' central emitter and
// proves that a request cannot substitute its own owner for the admitted owner.
func TestIPRequestSubscriberID(t *testing.T) {
	for _, id := range []string{"opaque/owner-65543", ""} {
		for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
			t.Run(id+"/"+family.String(), func(t *testing.T) {
				events := make(chan IPEvent, 1)
				decisions := make(chan ipResponseMsg, 1)
				decisions <- ipResponseMsg{accept: true, family: family}
				session := &pppSession{
					subscriberID: id,
					ipEventsOut:  events,
					ipRespCh:     decisions,
				}
				request := EventIPRequest{SubscriberID: "stale-request-owner", Family: family}
				decision, ok := session.awaitIPDecision(request, family)
				if !ok {
					t.Fatal("address decision did not complete")
				}
				if !decision.accept {
					t.Fatal("address decision was not preserved")
				}
				request, ok = (<-events).(EventIPRequest)
				if !ok {
					t.Fatal("emitter did not publish an address request")
				}
				if request.SubscriberID != id {
					t.Fatalf("address subscriber ID = %q, want %q", request.SubscriberID, id)
				}
			})
		}
	}
}

// TestDriverAdmissionSubscriberID queues a real session and observes its
// first authentication request, proving admission copies the caller's opaque ID.
func TestDriverAdmissionSubscriberID(t *testing.T) {
	registry := newPipeRegistry()
	installPipeRegistry(t, registry)
	pair := newPipePair(registry, 63001)
	defer closeConn(pair.peerEnd)
	ops, _, _ := newFakeOps()
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	defer driver.Stop()

	const id = "opaque/admitted-owner-65543"
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)})
	driver.SessionsIn() <- testSessionStart(StartSession{
		LCP:                 testLCPConfig(),
		SubscriberID:        id,
		TunnelID:            63,
		SessionID:           1,
		LNSMode:             true,
		MaxMRU:              1500,
		DisableIPCP:         true,
		DisableIPv6CP:       true,
		ProxyLCPInitialRecv: stream,
		ProxyLCPLastSent:    stream,
		ProxyLCPLastRecv:    stream,
	}, &fakeBackend{}, ops, 63001, 63999, 1)

	select {
	case event := <-driver.AuthEventsOut():
		request, ok := event.(EventAuthRequest)
		if !ok {
			t.Fatalf("first authentication event = %T, want EventAuthRequest", event)
		}
		if request.SubscriberID != id {
			t.Fatalf("admitted subscriber ID = %q, want %q", request.SubscriberID, id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("admitted session did not request authentication")
	}
}
