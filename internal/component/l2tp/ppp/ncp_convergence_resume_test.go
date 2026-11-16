// Design: docs/research/l2tpv2-ze-integration.md -- resumed initial NCP convergence
// Related: ncp.go -- runNCPPhase

package ppp

import (
	"net/netip"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// Resuming Configure while another family has termination grace must restore
// the overall convergence bound. Valid Naks renew the RFC Restart budget but
// cannot keep the renewed negotiation alive indefinitely.
func TestNCPConvergenceResumesDuringTerminationGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.disableIPCP = false
		session.disableIPv6CP = false
		session.ipTimeout = time.Second
		session.echoInterval = time.Hour
		authEvents := make(chan AuthEvent, 16)
		session.authEventsOut = authEvents
		ips := make(chan IPEvent, 2)
		session.ipEventsOut = ips
		peer := runLifecycleSession(t, session)
		openLifecycleLCP(t, peer)
		awaitAuthRequest(t, authEvents)
		session.authRespCh <- authResponseMsg{accept: true}
		select {
		case event := <-ips:
			request, ok := event.(EventIPRequest)
			if !ok || request.Family != AddressFamilyIPv4 {
				t.Fatalf("IPv4 address request = %+v", event)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("missing IPv4 address request")
		}
		session.ipRespCh <- ipResponseMsg{accept: true, family: AddressFamilyIPv4, local: netip.MustParseAddr("192.0.2.1"), peer: netip.MustParseAddr("192.0.2.2")}
		request4 := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		select {
		case event := <-ips:
			request, ok := event.(EventIPRequest)
			if !ok || request.Family != AddressFamilyIPv6 {
				t.Fatalf("IPv6 address request = %+v", event)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("missing IPv6 address request")
		}
		session.ipRespCh <- ipResponseMsg{accept: true, family: AddressFamilyIPv6}
		request6 := readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
		options6, err := parseIPv6CPOptions(request6.Data)
		if err != nil {
			t.Fatal(err)
		}
		peerID := options6.InterfaceID
		peerID[0] = 2
		peerID[7] ^= 0xff
		peerOptions := append([]byte{IPv6CPOptInterfaceID, 10}, peerID[:]...)
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x51, []byte{IPCPOptIPAddress, 6, 192, 0, 2, 2})
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck)
		writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest, 0x52, peerOptions)
		readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck)
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck, request4.Identifier, request4.Data)
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPTerminateRequest, 0x71, nil)
		readLifecyclePacket(t, peer, ProtoIPCP, LCPTerminateAck)
		writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck, request6.Identifier, request6.Data)
		synctest.Wait()
		// Let the original convergence deadline pass with only termination
		// grace outstanding. No peer Ack of our Terminate-Ack is invented.
		time.Sleep(1100 * time.Millisecond)
		synctest.Wait()
		for len(events) > 0 {
			switch event := (<-events).(type) {
			case EventSessionUp:
				t.Fatal("session completed before termination grace")
			case EventSessionDown:
				t.Fatalf("termination grace ended PPP: %+v", event)
			}
		}
		renewedAt := time.Now()
		writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest, 0x72, peerOptions)
		request := readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
		readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck)
		for attempt := byte(1); attempt <= 9; attempt++ {
			time.Sleep(100 * time.Millisecond)
			// Every Nak correlates with the last actual request and offers
			// a different nonzero identifier, keeping RCN/IRC/SCR active.
			suggested := [8]byte{2, 0, 0, 0, 0, 0, 0, 100 + attempt}
			options := append([]byte{IPv6CPOptInterfaceID, 10}, suggested[:]...)
			writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureNak, request.Identifier, options)
			request = readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
		}
		down := awaitLifecycleDown(t, events)
		if !strings.Contains(down.Reason, "ncp: timeout after 1s") {
			t.Fatalf("renewed convergence ended for %q", down.Reason)
		}
		if elapsed := time.Since(renewedAt); elapsed > session.ipTimeout+100*time.Millisecond {
			t.Fatalf("Naks extended convergence to %s", elapsed)
		}
	})
}
