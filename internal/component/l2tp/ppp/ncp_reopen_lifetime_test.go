// Design: docs/research/l2tpv2-ze-integration.md -- independent NCP lifetimes
// Related: ncp.go -- performNCPAction TLD, withdrawNCPFamily, publishNCPRelease, onNCPOpened
// Related: ncp_restart_lifetime_test.go -- the lifecycle helpers this file reuses
//
// Goal: prove AC-8 through the session's real run loop: after the peer
// terminates one NCP once the session is up, Ze withdraws that family and
// publishes EventSessionIPReleased for it; a new peer Configure-Request then
// renegotiates the family with the address or interface identifier the
// session already holds, with no second request to the address handler, and
// Ze publishes EventSessionIPAssigned for the same family again.
//
// Method: synctest and a net.Pipe channel, as ncp_restart_lifetime_test.go;
// one family is enabled per subtest, so the event stream holds only its
// events. Everything is observed across the boundary: the frames on the pipe,
// the events, the address requests and the fake backend's recorded calls,
// never session fields.

package ppp

import (
	"bytes"
	"net"
	"testing"
	"testing/synctest"
	"time"
)

// reopenFamilyLifecycle drives one family of a running session up to Opened
// and plays its peer.
type reopenFamilyLifecycle struct {
	family  AddressFamily
	peer    net.Conn
	request LCPPacket // Ze's first Configure-Request, which the peer acknowledged
	options []byte    // the peer's own Configure-Request options
}

// startReopenFamily enables family alone, runs the session, and opens LCP,
// authentication and the family's NCP, up to session-up.
func startReopenFamily(t *testing.T, session *pppSession, family AddressFamily, events <-chan Event) (
	*reopenFamilyLifecycle, <-chan IPEvent) {
	t.Helper()
	authEvents, ips := prepareIPCPLifecycle(session)
	if family == AddressFamilyIPv6 {
		session.disableIPCP = true
		session.disableIPv6CP = false
	}
	peer := runLifecycleSession(t, session)
	if family == AddressFamilyIPv4 {
		request := negotiateIPCPLifecycle(t, peer, authEvents, ips)
		openLifecycleIPCP(t, peer, request, events)
		address := restartLifetimePeer.As4()
		return &reopenFamilyLifecycle{family: family, peer: peer, request: request,
			options: append([]byte{IPCPOptIPAddress, 6}, address[:]...)}, ips
	}
	openLifecycleLCP(t, peer)
	answerAuthRequestForTest(t, awaitAuthRequest(t, authEvents), authResponseMsg{accept: true})
	select {
	case event := <-ips:
		request, ok := event.(EventIPRequest)
		if !ok {
			t.Fatalf("address event %T, want EventIPRequest", event)
		}
		if request.Family != AddressFamilyIPv6 {
			t.Fatalf("address request for %s, want IPv6", request.Family)
		}
		answerIPRequestForTest(t, request, ipResponseMsg{accept: true, family: AddressFamilyIPv6})
	case <-time.After(5 * time.Second):
		t.Fatal("missing IPv6 address request")
	}
	request := readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
	ours, err := parseIPv6CPOptions(request.Data)
	if err != nil {
		t.Fatal(err)
	}
	peerID := ours.InterfaceID
	peerID[7] ^= 0xff
	peerID[0] = 0x02
	options := append([]byte{IPv6CPOptInterfaceID, 10}, peerID[:]...)
	writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck, request.Identifier, request.Data)
	writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest, 0x52, options)
	readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck)
	awaitLifecycleUp(t, events)
	return &reopenFamilyLifecycle{family: family, peer: peer, request: request, options: options}, ips
}

// awaitNetworkEvent returns the next release or assignment event, failing on
// a session-down and on a deadline. Other events are passed over.
func awaitNetworkEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case event := <-events:
			switch event.(type) {
			case EventSessionIPReleased, EventSessionIPAssigned:
				return event
			case EventSessionDown:
				t.Fatalf("session went down while one family was withdrawn: %+v", event)
			}
		case <-deadline:
			t.Fatal("no release or assignment event")
		}
	}
}

// TestNCPReopenAfterWithdrawal proves AC-8 for each family: Opened, the
// peer's Terminate-Request draws a Terminate-Ack with its Identifier and an
// EventSessionIPReleased for that family (for IPv4 after the installed
// address was removed from pppN); one Restart time later, in Stopped, the
// peer's new Configure-Request draws Ze's Configure-Request with the same
// options as before (the held local address or interface identifier) and the
// Configure-Ack; the peer's Ack of it reopens the family and Ze publishes
// EventSessionIPAssigned for it with the held address (IPv4: the same local
// and peer, re-added to pppN) or the peer's interface identifier (IPv6),
// without a second address request. NP mode is not observable here: the
// kernel handle sets it inside RemoveAddress and AddAddressP2P
// (kernel/session.go), which this fixture records as the address calls.
//
// MUTATION: delete the publishNCPRelease call from the LCPActTLD case of
// performNCPAction (ncp.go): the first network event is the reassignment, not
// the release, and the release check fails.
// MUTATION: make the LCPActTLD case of performNCPAction (ncp.go) skip
// withdrawNCPFamily: the IPv4 subtest records no address removal and fails.
func TestNCPReopenAfterWithdrawal(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				session, _, events := newRFC1661Session(LCPStateInitial)
				session.ipTimeout = 30 * time.Second
				backend, ok := testHandleFor(session).backend.(*fakeBackend)
				if !ok {
					t.Fatal("session backend is not the recording fake")
				}
				lifecycle, ips := startReopenFamily(t, session, family, events)
				proto := ncpProto(family)
				installed := backend.P2PCalls()

				writeLifecyclePacket(t, lifecycle.peer, proto, LCPTerminateRequest, 0x71, nil)
				if ack := readLifecyclePacket(t, lifecycle.peer, proto, LCPTerminateAck); ack.Identifier != 0x71 {
					t.Fatalf("Terminate-Ack Identifier = %#02x, want the request's 0x71", ack.Identifier)
				}
				released, isRelease := awaitNetworkEvent(t, events).(EventSessionIPReleased)
				if !isRelease {
					t.Fatal("the first network event after the Terminate-Request is not EventSessionIPReleased")
				}
				if released.Family != family {
					t.Fatalf("released family %s, want %s", released.Family, family)
				}
				if family == AddressFamilyIPv4 {
					removed := backend.AddrRemoveCalls()
					if len(installed) != 1 || len(removed) != 1 || removed[0].cidr != installed[0].local {
						t.Fatalf("installed %+v, removed %+v: want the one installed local address removed", installed, removed)
					}
				}

				// One Restart time takes the family from Stopping to Stopped.
				time.Sleep(testLCPConfig().RestartTimer() + time.Millisecond)
				writeLifecyclePacket(t, lifecycle.peer, proto, LCPConfigureRequest, 0x5b, lifecycle.options)
				fresh := readLifecyclePacket(t, lifecycle.peer, proto, LCPConfigureRequest)
				if !bytes.Equal(fresh.Data, lifecycle.request.Data) {
					t.Fatalf("renegotiation Configure-Request carries % x, want the held % x", fresh.Data, lifecycle.request.Data)
				}
				if ack := readLifecyclePacket(t, lifecycle.peer, proto, LCPConfigureAck); ack.Identifier != 0x5b {
					t.Fatalf("Configure-Ack Identifier = %#02x, want the request's 0x5b", ack.Identifier)
				}
				writeLifecyclePacket(t, lifecycle.peer, proto, LCPConfigureAck, fresh.Identifier, fresh.Data)

				assigned, isAssignment := awaitNetworkEvent(t, events).(EventSessionIPAssigned)
				if !isAssignment {
					t.Fatal("the network event after the reopen is not EventSessionIPAssigned")
				}
				if assigned.Family != family {
					t.Fatalf("reassigned family %s, want %s", assigned.Family, family)
				}
				switch family {
				case AddressFamilyIPv4:
					if assigned.Local != restartLifetimeLocal || assigned.Peer != restartLifetimePeer {
						t.Fatalf("reassigned %s/%s, want the held %s/%s", assigned.Local, assigned.Peer,
							restartLifetimeLocal, restartLifetimePeer)
					}
					reinstalled := backend.P2PCalls()
					if len(reinstalled) != 2 || reinstalled[1] != installed[0] {
						t.Fatalf("address calls %+v, want the held pair %+v added once more", reinstalled, installed[0])
					}
				case AddressFamilyIPv6:
					if !bytes.Equal(assigned.InterfaceID[:], lifecycle.options[2:]) {
						t.Fatalf("reassigned interface identifier % x, want the peer's % x",
							assigned.InterfaceID, lifecycle.options[2:])
					}
				default:
					t.Fatalf("unexpected family %s", family)
				}
				select {
				case event := <-ips:
					t.Fatalf("the reopen asked the address handler again: %+v", event)
				default:
				}
			})
		})
	}
}
