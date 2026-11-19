// Design: docs/architecture/l2tp/subscriber-session-model.md -- authenticated subscriber assignment lifetimes

package ppp

import (
	"bytes"
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// Two real CHAP and IPCP rounds run on the same admitted PPPoE worker. Each
// packet carries only its accepted request's immutable routes. A reply to the
// previous request cannot replace the second profile despite sharing identity.
func TestPPPoEFramedRoutesFollowExactAuthenticationRound(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer, _ := frameLeaseStart(t, 21)
	t.Cleanup(func() { closeConn(peer); driver.Stop() })
	start.PPPoE = true
	start.AuthMethod = AuthMethodCHAPMD5
	start.AuthRequired = true
	start.DisableIPCP = false
	driver.SessionsIn() <- start
	openLifecycleLCP(t, peer)
	first := readFramedRouteAuthRequest(t, driver, peer)
	firstRoute := l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Metric: 11}
	callerRoutes := []l2tpevents.FramedRoute{firstRoute}
	if err := first.StageFramedRoutes(callerRoutes); err != nil {
		t.Fatal(err)
	}
	callerRoutes[0].Metric = 99
	firstAssignment := awaitFramedRouteAssignment(t, driver, peer, first)
	if !slices.Equal(firstAssignment.FramedRoutes, []l2tpevents.FramedRoute{firstRoute}) {
		t.Errorf("first authenticated assignment routes = %+v, want the original profile %+v", firstAssignment.FramedRoutes, firstRoute)
	}
	firstAssignment.Acknowledge()
	awaitLifecycleUp(t, driver.EventsOut())

	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, 0x42, nil)
	down, ok := waitForEvent(t, driver.EventsOut(), time.Second).(EventLCPDown)
	if !ok || !down.NetworkPhase || down.Control != first.Control {
		t.Fatalf("LCP-down = %+v, want the exact first network", down)
	}
	down.Acknowledge()
	configure := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
	readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck)
	writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureAck, configure.Identifier, configure.Data)
	second := readFramedRouteAuthRequest(t, driver, peer)
	if second.Control != first.Control || second.SubscriberID != first.SubscriberID {
		t.Fatal("renegotiation did not reuse the original admitted PPPoE worker")
	}
	secondRoute := l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("2001:db8:12::/48"), Metric: 22}
	if err := second.StageFramedRoutes([]l2tpevents.FramedRoute{secondRoute}); err != nil {
		t.Fatal(err)
	}
	if err := first.StageFramedRoutes([]l2tpevents.FramedRoute{firstRoute}); !errors.Is(err, ErrRequestExpired) {
		t.Errorf("retired request staging = %v, want request expired", err)
	}
	if err := driver.AuthResponse(first, true, "", nil); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("retired authentication reply = %v, want request expired", err)
	}
	secondAssignment := awaitFramedRouteAssignment(t, driver, peer, second)
	if secondAssignment.NetworkLifetime == firstAssignment.NetworkLifetime {
		t.Error("renegotiation retained the first network lifetime")
	}
	if !slices.Equal(secondAssignment.FramedRoutes, []l2tpevents.FramedRoute{secondRoute}) {
		t.Errorf("second authenticated assignment routes = %+v, want only %+v", secondAssignment.FramedRoutes, secondRoute)
	}
	secondAssignment.Acknowledge()
	awaitLifecycleUp(t, driver.EventsOut())
}

func readFramedRouteAuthRequest(t *testing.T, driver *Driver, peer net.Conn) EventAuthRequest {
	t.Helper()
	if _, ok := waitForEvent(t, driver.EventsOut(), time.Second).(EventLCPUp); !ok {
		t.Fatal("PPPoE subscriber did not reach LCP-up")
	}
	challenge := readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeChallenge)
	value := bytes.Repeat([]byte{0x33}, chapMD5DigestLen)
	response := append([]byte{byte(len(value))}, value...)
	response = append(response, "subscriber"...)
	writeLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeResponse, challenge.Identifier, response)
	request := readProviderLifetimeAuthRequest(t, driver.AuthEventsOut())
	if request.Method != AuthMethodCHAPMD5 || request.Identifier != challenge.Identifier {
		t.Fatal("the provider did not receive the actual CHAP request")
	}
	return request
}

func awaitFramedRouteAssignment(t *testing.T, driver *Driver, peer net.Conn, request EventAuthRequest) EventSessionIPAssigned {
	t.Helper()
	if err := driver.AuthResponse(request, true, "", nil); err != nil {
		t.Fatal(err)
	}
	readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeSuccess)
	select {
	case event := <-driver.AuthEventsOut():
		success, ok := event.(eventAuthSuccess)
		if !ok || success.Control != request.Control {
			t.Fatalf("actual accepted authentication = %T, want the original success", event)
		}
	case <-time.After(time.Second):
		t.Fatal("the actual accepted decision was not consumed")
	}
	session := &l2tpBirthProducerSession{driver: driver, peer: peer, control: request.Control}
	return prepareAuthAdmissionIPCP(t, session)
}
