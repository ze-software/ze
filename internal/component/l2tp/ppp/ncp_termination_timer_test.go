// Design: docs/research/l2tpv2-ze-integration.md -- independent NCP Restart timers
// Related: ncp.go -- runNCPPhase and NCP transitions

package ppp

import (
	"net/netip"
	"testing"
	"testing/synctest"
	"time"
)

// A normal peer sends Terminate-Request and receives our Ack; it sends no Ack
// of that Ack. Drive the real session before and after initial convergence,
// then require the family's grace expiry without losing the independent NCP.
func TestNCPTerminationGracePreservesIndependentFamily(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		for _, active := range []bool{false, true} {
			name := "initial"
			if active {
				name = "active"
			}
			t.Run(family.String()+"/"+name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					session, _, events := newRFC1661Session(LCPStateInitial)
					session.disableIPCP = false
					session.disableIPv6CP = false
					// The convergence deadline must not shorten the separate
					// RFC termination grace after negotiation has settled.
					session.ipTimeout = time.Second
					session.echoInterval = time.Hour
					authEvents := make(chan AuthEvent, 16)
					session.authEventsOut = authEvents
					ips := make(chan IPEvent, 2)
					session.ipEventsOut = ips
					peer := runLifecycleSession(t, session)
					openLifecycleLCP(t, peer)
					authRequest := awaitAuthRequest(t, authEvents)
					answerAuthRequestForTest(t, authRequest, authResponseMsg{accept: true})
					var fixtureAddressRequest1 EventIPRequest
					select {
					case event := <-ips:
						request, ok := event.(EventIPRequest)
						fixtureAddressRequest1 = request
						if !ok || request.Family != AddressFamilyIPv4 {
							t.Fatalf("IPv4 address request = %+v", event)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("missing IPv4 address request")
					}
					answerIPRequestForTest(t, fixtureAddressRequest1, ipResponseMsg{accept: true, family: AddressFamilyIPv4, local: netip.MustParseAddr("192.0.2.1"), peer: netip.MustParseAddr("192.0.2.2")})
					// Both initial requests precede peer-frame dispatch.
					request4 := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
					var fixtureAddressRequest2 EventIPRequest
					select {
					case event := <-ips:
						request, ok := event.(EventIPRequest)
						fixtureAddressRequest2 = request
						if !ok || request.Family != AddressFamilyIPv6 {
							t.Fatalf("IPv6 address request = %+v", event)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("missing IPv6 address request")
					}
					answerIPRequestForTest(t, fixtureAddressRequest2, ipResponseMsg{accept: true, family: AddressFamilyIPv6})
					request6 := readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
					options6, err := parseIPv6CPOptions(request6.Data)
					if err != nil {
						t.Fatal(err)
					}
					peerID := options6.InterfaceID
					peerID[7] ^= 0xff
					peerID[0] = 0x02
					writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x51, []byte{IPCPOptIPAddress, 6, 192, 0, 2, 2})
					readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck)
					writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest, 0x52, append([]byte{IPv6CPOptInterfaceID, 10}, peerID[:]...))
					readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck)
					closedRequest, otherRequest := request4, request6
					if family == AddressFamilyIPv6 {
						closedRequest, otherRequest = request6, request4
					}
					writeLifecyclePacket(t, peer, ncpProto(family), LCPConfigureAck, closedRequest.Identifier, closedRequest.Data)
					if active {
						writeLifecyclePacket(t, peer, ncpProto(otherNCPFamily(family)), LCPConfigureAck, otherRequest.Identifier, otherRequest.Data)
						awaitLifecycleUp(t, events)
					}
					writeLifecyclePacket(t, peer, ncpProto(family), LCPTerminateRequest, 0x71, nil)
					ack := readLifecyclePacket(t, peer, ncpProto(family), LCPTerminateAck)
					if ack.Identifier != 0x71 {
						t.Fatalf("Terminate-Ack identifier = %d", ack.Identifier)
					}
					if !active {
						writeLifecyclePacket(t, peer, ncpProto(otherNCPFamily(family)), LCPConfigureAck, otherRequest.Identifier, otherRequest.Data)
						awaitLifecycleUp(t, events)
					}
					time.Sleep(2 * session.lcpConfig.RestartTimer())
					synctest.Wait()
					if got := session.ncpState(family); got != LCPStateStopped {
						t.Fatalf("terminated family = %s, want Stopped after actual grace timer", got)
					}
					if got := session.ncpState(otherNCPFamily(family)); got != LCPStateOpened {
						t.Fatalf("independent family = %s, want Opened", got)
					}
					for len(events) > 0 {
						if event, ok := (<-events).(EventSessionDown); ok {
							t.Fatalf("normal family termination ended PPP: %+v", event)
						}
					}
				})
			})
		}
	}
}
