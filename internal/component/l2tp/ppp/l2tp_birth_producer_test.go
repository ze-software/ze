// Design: docs/architecture/l2tp/subscriber-session-model.md -- original logical birth at PPP lifecycle boundaries

package ppp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/subscriberid"
)

// The actual Driver admission must retain the original transport identity
// throughout LCP, authentication, usable network lifetime and final withdrawal.
// MUTATION: omitting identity stamping in bindEventControl loses the birth and
// canonical ID even though its exact SessionControl still remains useful.
func TestDriverL2TPLifecyclePreservesOriginalBirth(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer, owner := frameLeaseStart(t, 9)
	t.Cleanup(func() {
		closeConn(peer)
		driver.Stop()
	})
	start.L2TPBirth = birth
	start.SubscriberID = birth.String()
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	first := waitForEvent(t, driver.EventsOut(), time.Second)
	opened, ok := first.(EventLCPUp)
	if !ok {
		t.Fatalf("first actual lifecycle event = %T, want LCP-up", first)
	}
	control := opened.Control
	if control == (SessionControl{}) {
		t.Fatal("actual admission omitted its control")
	}
	if control.Attempt() != start.Attempt {
		t.Fatal("actual admission substituted its original transport attempt")
	}
	if control.SubscriberID() != start.SubscriberID {
		t.Fatal("actual control changed the canonical subscriber ID")
	}
	if opened.Birth != birth || opened.SubscriberID != start.SubscriberID {
		t.Errorf("LCP-up identity = %v / %q, want original %v / %q", opened.Birth, opened.SubscriberID, birth, start.SubscriberID)
	}
	select {
	case event := <-driver.AuthEventsOut():
		request, ok := event.(EventAuthRequest)
		if !ok {
			t.Fatalf("actual authentication event = %T, want request", event)
		}
		if request.Control != control || request.SubscriberID != start.SubscriberID {
			t.Fatal("authentication request changed the admitted identity")
		}
		if err := driver.AuthResponse(request, true, "", nil); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("actual admission did not request authentication")
	}
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	up, ok := event.(EventSessionUp)
	if !ok {
		t.Fatalf("authenticated lifecycle event = %T, want session-up", event)
	}
	if up.Control != control {
		t.Fatal("session-up changed the original admitted control")
	}
	if up.Birth != birth || up.SubscriberID != start.SubscriberID {
		t.Errorf("session-up identity = %v / %q, want original %v / %q", up.Birth, up.SubscriberID, birth, start.SubscriberID)
	}
	frameLeaseEcho(t, peer)
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	event = waitForEvent(t, driver.EventsOut(), time.Second)
	down, ok := event.(EventSessionDown)
	if !ok {
		t.Fatalf("stopped lifecycle event = %T, want session-down", event)
	}
	if down.Control != control {
		t.Fatal("session-down changed the original admitted control")
	}
	if down.Birth != birth || down.SubscriberID != start.SubscriberID {
		t.Errorf("session-down identity = %v / %q, want original %v / %q", down.Birth, down.SubscriberID, birth, start.SubscriberID)
	}
}

// Refusal must carry the incoming birth and exact attempt without inventing an
// admitted control. The ordinary invalid-LCP boundary supplies the refusal.
// MUTATION: omitting identity in sendRejection strips the refused original.
func TestDriverL2TPRefusalPreservesOriginalBirth(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer, owner := frameLeaseStart(t, 10)
	t.Cleanup(func() {
		closeConn(peer)
		driver.Stop()
	})
	start.LCP = LCPConfig{}
	start.L2TPBirth = birth
	start.SubscriberID = birth.String()
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	refused, ok := event.(EventSessionRejected)
	if !ok {
		t.Fatalf("actual invalid-LCP event = %T, want refusal", event)
	}
	if refused.Attempt != start.Attempt {
		t.Fatal("refusal substituted the original incoming attempt")
	}
	if _, admitted := refused.Attempt.Control(); admitted {
		t.Fatal("refusal minted an admitted control")
	}
	if refused.Birth != birth || refused.SubscriberID != start.SubscriberID {
		t.Fatalf("refusal identity = %v / %q, want original %v / %q", refused.Birth, refused.SubscriberID, birth, start.SubscriberID)
	}
}

// Actual IPCP must publish its original birth before the same admission becomes
// usable. The peer completes the real address exchange and answers useful LCP.
// MUTATION: dropping identity in sendIPAssigned strips the assignment's owner.
func TestDriverL2TPIPAssignmentPreservesOriginalBirth(t *testing.T) {
	session := startL2TPBirthProducerSession(t, true, 0)
	select {
	case event := <-session.driver.IPEventsOut():
		request, ok := event.(EventIPRequest)
		if !ok {
			t.Fatalf("actual address event = %T, want request", event)
		}
		if request.Control != session.control {
			t.Fatal("address request changed the original admitted control")
		}
		if request.SubscriberID != session.control.SubscriberID() {
			t.Fatal("address request changed the canonical subscriber ID")
		}
		if request.Family != AddressFamilyIPv4 {
			t.Fatalf("address request family = %v, want IPv4", request.Family)
		}
		if err := session.driver.IPResponse(request, IPResponseArgs{
			Accept: true, Family: AddressFamilyIPv4,
			Local: ipcpTestLocal, Peer: ipcpTestPeer,
			DNSPrimary: ipcpTestDNS1, DNSSecondary: ipcpTestDNS2,
		}); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("actual admission did not request an address")
	}

	exchange := &ncpTestDriver{driver: session.driver, peer: session.peer}
	exchange.completeIPCP(t)
	event := waitForEvent(t, session.driver.EventsOut(), time.Second)
	assigned, ok := event.(EventSessionIPAssigned)
	if !ok {
		t.Fatalf("actual IPCP event = %T, want assignment", event)
	}
	// This test is the transport consumer and MUST acknowledge this exact event
	// before waiting for the producer's usable-session notification.
	assigned.Acknowledge()
	if assigned.Control != session.control {
		t.Fatal("assignment changed the original admitted control")
	}
	if assigned.Family != AddressFamilyIPv4 {
		t.Fatalf("assigned family = %v, want IPv4", assigned.Family)
	}
	if assigned.Local != ipcpTestLocal {
		t.Fatalf("assigned local address = %v, want %v", assigned.Local, ipcpTestLocal)
	}
	if assigned.Peer != ipcpTestPeer {
		t.Fatalf("assigned peer address = %v, want %v", assigned.Peer, ipcpTestPeer)
	}
	if assigned.Birth != session.birth {
		t.Errorf("assignment birth = %v, want original %v", assigned.Birth, session.birth)
	}
	if assigned.SubscriberID != session.control.SubscriberID() {
		t.Errorf("assignment subscriber ID = %q, want original %q", assigned.SubscriberID, session.control.SubscriberID())
	}
	session.requireUp(t)
	frameLeaseEcho(t, session.peer)
}

// Peer-driven LCP renegotiation must withdraw the original network lifetime's
// identity, then retain the same admission through acknowledged restart and Up.
// MUTATION: dropping identity in bindEventControl strips the LCP-down's owner.
func TestDriverL2TPLCPDownPreservesOriginalBirth(t *testing.T) {
	session := startL2TPBirthProducerSession(t, false, 0)
	session.requireUp(t)
	// The proxy's local Magic-Number is 0xCAFEBABE, so the new peer offer
	// uses the existing distinct peer value rather than triggering loopback.
	options := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(proxyTransmitPeerMagic)})
	writeLifecyclePacket(t, session.peer, ProtoLCP, LCPConfigureRequest, 0x51, options)
	event := waitForEvent(t, session.driver.EventsOut(), time.Second)
	down, ok := event.(EventLCPDown)
	if !ok {
		t.Fatalf("actual renegotiation event = %T, want LCP-down", event)
	}
	// The consumer MUST acknowledge the original withdrawal before replacement
	// LCP/authentication can proceed. The copied event remains immutable.
	down.Acknowledge()
	if !down.NetworkPhase {
		t.Fatal("renegotiation did not end an actual network lifetime")
	}
	if down.Control != session.control {
		t.Fatal("LCP-down changed the original admitted control")
	}
	if down.Birth != session.birth {
		t.Errorf("LCP-down birth = %v, want original %v", down.Birth, session.birth)
	}
	if down.SubscriberID != session.control.SubscriberID() {
		t.Errorf("LCP-down subscriber ID = %q, want original %q", down.SubscriberID, session.control.SubscriberID())
	}

	request := readLifecyclePacket(t, session.peer, ProtoLCP, LCPConfigureRequest)
	readLifecyclePacket(t, session.peer, ProtoLCP, LCPConfigureAck)
	writeLifecyclePacket(t, session.peer, ProtoLCP, LCPConfigureAck, request.Identifier, request.Data)
	event = waitForEvent(t, session.driver.EventsOut(), time.Second)
	opened, ok := event.(EventLCPUp)
	if !ok {
		t.Fatalf("renegotiated lifecycle event = %T, want LCP-up", event)
	}
	if opened.Control != session.control {
		t.Fatal("renegotiation changed the original admitted control")
	}
	session.acceptAuth(t)
	session.requireUp(t)
	assertProxyTransmitEcho(t, &ncpTestDriver{peer: session.peer}, proxyTransmitPeerMagic)
}

// A timer-generated Echo-Request and its real peer reply must measure RTT for
// the original admission. The peer also requests a reply after the measurement.
// MUTATION: dropping identity in bindEventControl strips the RTT sample's owner.
func TestDriverL2TPEchoRTTPreservesOriginalBirth(t *testing.T) {
	session := startL2TPBirthProducerSession(t, false, time.Second)
	session.requireUp(t)
	request := readLifecyclePacket(t, session.peer, ProtoLCP, LCPEchoRequest)
	var magic [4]byte
	binary.BigEndian.PutUint32(magic[:], 0xCAFEBABE)
	writeLifecyclePacket(t, session.peer, ProtoLCP, LCPEchoReply, request.Identifier, magic[:])
	event := waitForEvent(t, session.driver.EventsOut(), time.Second)
	echo, ok := event.(EventEchoRTT)
	if !ok {
		t.Fatalf("actual echo event = %T, want RTT", event)
	}
	if echo.Control != session.control {
		t.Fatal("RTT sample changed the original admitted control")
	}
	if echo.RTT <= 0 {
		t.Fatalf("actual peer round-trip = %s, want a positive measurement", echo.RTT)
	}
	if echo.Birth != session.birth {
		t.Errorf("RTT birth = %v, want original %v", echo.Birth, session.birth)
	}
	if echo.SubscriberID != session.control.SubscriberID() {
		t.Errorf("RTT subscriber ID = %q, want original %q", echo.SubscriberID, session.control.SubscriberID())
	}
	frameLeaseEcho(t, session.peer)
}

type l2tpBirthProducerSession struct {
	driver  *Driver
	peer    net.Conn
	birth   subscriberid.L2TPBirth
	control SessionControl
}

// startL2TPBirthProducerSession admits through the actual Driver. Cleanup closes
// the peer and stops the Driver, which alone owns the transferred data plane.
func startL2TPBirthProducerSession(t *testing.T, withIPCP bool, echoInterval time.Duration) *l2tpBirthProducerSession {
	t.Helper()
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer, owner := frameLeaseStart(t, 11)
	t.Cleanup(func() {
		closeConn(peer)
		driver.Stop()
	})
	start.L2TPBirth = birth
	start.SubscriberID = birth.String()
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.DisableIPCP = !withIPCP
	start.EchoInterval = echoInterval
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	opened, ok := event.(EventLCPUp)
	if !ok {
		t.Fatalf("first actual lifecycle event = %T, want LCP-up", event)
	}
	if opened.Control == (SessionControl{}) {
		t.Fatal("actual admission omitted its control")
	}
	if opened.Control.Attempt() != start.Attempt {
		t.Fatal("actual admission substituted the original attempt")
	}
	if opened.Control.SubscriberID() != start.SubscriberID {
		t.Fatal("actual admission changed the canonical subscriber ID")
	}
	session := &l2tpBirthProducerSession{driver: driver, peer: peer, birth: birth, control: opened.Control}
	session.acceptAuth(t)
	return session
}

// acceptAuth completes the actual decision of this admission and network
// lifetime. The consumer MUST drain its original informational success before
// expecting a replacement lifetime's next request.
func (s *l2tpBirthProducerSession) acceptAuth(t *testing.T) {
	t.Helper()
	select {
	case event := <-s.driver.AuthEventsOut():
		request, ok := event.(EventAuthRequest)
		if !ok {
			t.Fatalf("actual authentication event = %T, want request", event)
		}
		if request.Control != s.control {
			t.Fatal("authentication request changed the original admitted control")
		}
		if request.SubscriberID != s.control.SubscriberID() {
			t.Fatal("authentication request changed the canonical subscriber ID")
		}
		if err := s.driver.AuthResponse(request, true, "", nil); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("actual admission did not request authentication")
	}
	select {
	case event := <-s.driver.AuthEventsOut():
		success, ok := event.(eventAuthSuccess)
		if !ok {
			t.Fatalf("actual authentication completion = %T, want success", event)
		}
		if success.Control != s.control {
			t.Fatal("authentication success changed the original admitted control")
		}
	case <-time.After(time.Second):
		t.Fatal("actual admission did not report accepted authentication")
	}
}

// requireUp observes the useful network lifetime of the same admitted control.
func (s *l2tpBirthProducerSession) requireUp(t *testing.T) {
	t.Helper()
	event := waitForEvent(t, s.driver.EventsOut(), time.Second)
	up, ok := event.(EventSessionUp)
	if !ok {
		t.Fatalf("authenticated lifecycle event = %T, want session-up", event)
	}
	if up.Control != s.control {
		t.Fatal("session-up changed the original admitted control")
	}
}
