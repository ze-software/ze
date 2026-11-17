//go:build linux

// Design: docs/architecture/l2tp/subscriber-session-model.md -- Linux network-lifetime fixture operations
// Related: current_core_fixture_test.go -- shared genuine Driver and protocol owner
package pppoe

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func (fixture *fixtureCurrentPPP) allocate(t *testing.T, v4 ppp.EventSessionIPAssigned) {
	t.Helper()
	for range 2 {
		select {
		case event := <-fixture.driver.IPEventsOut():
			request, ok := event.(ppp.EventIPRequest)
			if !ok {
				t.Fatal("actual IP request missing")
			}
			answer := ppp.IPResponseArgs{Accept: true, Family: request.Family}
			if request.Family == ppp.AddressFamilyIPv4 {
				answer.Local = netip.MustParseAddr("192.0.2.1")
				answer.Peer = v4.Peer
				answer.DNSPrimary = v4.DNSPrimary
				answer.DNSSecondary = v4.DNSSecondary
			}
			if err := fixture.driver.IPResponse(request, answer); err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("actual IP request missing")
		}
	}
}
func (fixture *fixtureCurrentPPP) assigned(t *testing.T, want ppp.EventSessionIPAssigned) ppp.EventSessionIPAssigned {
	t.Helper()
	protocol := ppp.ProtoIPCP
	data := []byte{3, 6}
	if want.Family == ppp.AddressFamilyIPv4 {
		address := want.Peer.As4()
		data = append(data, address[:]...)
	} else {
		protocol = ppp.ProtoIPv6CP
		data = append([]byte{1, 10}, want.InterfaceID[:]...)
	}
	request := fixture.read(t, protocol, ppp.LCPConfigureRequest)
	fixture.send(t, protocol, ppp.LCPConfigureAck, request.Identifier, request.Data)
	fixture.send(t, protocol, ppp.LCPConfigureRequest, 0x31, data)
	fixture.read(t, protocol, ppp.LCPConfigureAck)
	assignment, ok := fixture.next(t).(ppp.EventSessionIPAssigned)
	if !ok || assignment.Control != fixture.control || assignment.Family != want.Family {
		t.Fatal("actual Driver assignment changed owner/family")
	}
	return assignment
}
func (fixture *fixtureCurrentPPP) restart(t *testing.T) ppp.EventLCPDown {
	t.Helper()
	fixture.send(t, ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x41, []byte{1, 4, 5, 212})
	down, ok := fixture.next(t).(ppp.EventLCPDown)
	if !ok || down.Control != fixture.control {
		t.Fatal("actual Driver omitted same-owner LCPDown")
	}
	return down
}
func (fixture *fixtureCurrentPPP) reopen(t *testing.T) {
	t.Helper()
	request := fixture.read(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
	fixture.read(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
	fixture.send(t, ppp.ProtoLCP, ppp.LCPConfigureAck, request.Identifier, request.Data)
	up, ok := fixture.next(t).(ppp.EventLCPUp)
	if !ok || up.Control != fixture.control {
		t.Fatal("actual Driver changed owner across LCP restart")
	}
	// LCPUp is emitted after the old network lifetime and LCP replies. Retire
	// only the peer helper's buffered old replies, never source input frames.
	fixture.packets = make(map[uint16][]ppp.LCPPacket)
}
