// Design: docs/research/l2tpv2-ze-integration.md -- independent NCP resource lifetimes
// Related: ncp.go -- performNCPAction TLD, publishNCPRelease
// Related: session_run.go -- readUnitFrames, the unit descriptor reader
// Related: unit_frames_test.go -- writeUnitNCPPacket
// Related: ncp_family_lifecycle_test.go -- ncpFamilyLifecycle

package ppp

import (
	"net"
	"testing"
	"time"
)

// TestNCPTerminateAfterSessionUpOnUnitFD proves AC-4 through the session's own
// run loop: a peer that closes one NCP after the session came up gets its
// Terminate-Ack, that family alone leaves pppN, and the transport learns it
// through one EventSessionIPReleased for that family.
//
// Method: a dual-stack session runs on the pipe driver with a net.Pipe as its
// unit descriptor. After EventSessionUp the test writes the NCP
// Terminate-Request into the unit pipe, never the channel, the way Linux
// ppp_input routes a frame below 0xc000 once the channel is attached to a
// unit. Everything is read where it crosses the session boundary: frames on
// the channel pipe, events on the driver's channel, address removals on the
// fake backend's locked recorder; no session field is read. The other family
// is then probed with an unused NCP Code on the unit: its Code-Reject on the
// channel proves the session goroutine handled every frame before it, so the
// empty event channel and the unchanged removal list at that point show the
// other family was not withdrawn. Kernel NP mode is not visible at this
// layer: RemoveAddress is the call the kernel adapter turns into the drop,
// proven by TestKernelNCPFamilyWithdrawalPrecedesAddressRemoval (kernel).
//
// Red today without the release: the release wait fails with no
// EventSessionIPReleased.
func TestNCPTerminateAfterSessionUpOnUnitFD(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			driverUnit, peerUnit := net.Pipe()
			t.Cleanup(func() { _ = peerUnit.Close() }) //nolint:errcheck // test cleanup

			td := newNCPTestDriverUnit(t, &StartSession{}, autoAcceptIP, discardLogger(), driverUnit)
			peerDone := make(chan struct{})
			go runParallelNCPPeer(t, td.peer, true, true, peerDone)
			assigned := awaitDualStackSessionUp(t, td)
			<-peerDone

			removalsBefore := td.backend.AddrRemoveCalls()

			// RFC 1661 Section 5.5: "Upon reception of a Terminate-Request, a
			// Terminate-Ack MUST be transmitted."
			writeUnitNCPPacket(t, peerUnit, ncpProto(family), LCPTerminateRequest, 0x77, nil)
			ack := td.readPeerNCPPacket(t, ncpProto(family))
			if ack.Code != LCPTerminateAck {
				t.Fatalf("answer to %s Terminate-Request on the unit has Code %d, want Terminate-Ack", family, ack.Code)
			}
			if ack.Identifier != 0x77 {
				t.Fatalf("Terminate-Ack Identifier = 0x%02x, want the request's 0x77", ack.Identifier)
			}

			released := awaitFamilyRelease(t, td)
			if released.Family != family {
				t.Fatalf("released family = %s, want %s", released.Family, family)
			}
			if released.TunnelID != 1 || released.SessionID != 1 {
				t.Fatalf("release names tunnel %d session %d, want 1/1", released.TunnelID, released.SessionID)
			}
			if released.NetworkLifetime != assigned[family].NetworkLifetime {
				t.Fatalf("release network lifetime = %+v, want the assignment's %+v",
					released.NetworkLifetime, assigned[family].NetworkLifetime)
			}

			// The release is published after the withdrawal, so the removal
			// is already recorded when the event is read.
			removals := td.backend.AddrRemoveCalls()[len(removalsBefore):]
			if family == AddressFamilyIPv4 {
				if len(removals) != 1 || removals[0].cidr != "10.0.0.1/32" {
					t.Fatalf("address withdrawals = %+v, want only the IPv4 address 10.0.0.1/32", removals)
				}
			} else {
				// The IPv6 withdrawal is the IPv6 service's own address, if this
				// driver's service started at all; the IPv4 pair MUST stay.
				for _, removal := range removals {
					if removal.cidr == "10.0.0.1/32" {
						t.Fatalf("address withdrawals = %+v: the IPv6 release removed the IPv4 pair", removals)
					}
				}
			}

			other := otherNCPFamily(family)
			// RFC 1332 Section 2 and RFC 5072 Section 3: an unknown Code draws
			// a Code-Reject, and its answer orders every earlier frame.
			writeUnitNCPPacket(t, peerUnit, ncpProto(other), unusedNCPCode, 0x42, []byte{1, 2, 3, 4})
			if reject := td.readPeerNCPPacket(t, ncpProto(other)); reject.Code != LCPCodeReject {
				t.Fatalf("other family's answer has Code %d, want Code-Reject", reject.Code)
			}
			select {
			case event := <-td.driver.EventsOut():
				t.Fatalf("withdrawing %s published a further %T, want nothing for %s", family, event, other)
			default:
			}
			if after := td.backend.AddrRemoveCalls()[len(removalsBefore):]; len(after) != len(removals) {
				t.Fatalf("address withdrawals grew to %+v after the %s release", after, family)
			}
		})
	}
}

// TestNCPReleaseSkipsUnpublishedFamily proves the guard publishNCPRelease
// reads is per family. The network lifetime is one per session, so the IPv4
// assignment sets it; an IPv6CP that reached Opened with no negotiated peer
// interface identifier published no assignment (onNCPOpened), so its TLD has
// nothing in the transport to release.
//
// Method: the ncpFamilyLifecycle helpers drive the real producers on a bare
// session: IPv4 opens and its assignment is acknowledged, IPv6 opens without
// a negotiated identifier, then each family takes a Terminate-Request through
// handleFrame. Only the events channel is read. Red under a session-wide
// guard (the network lifetime alone): the IPv6 Terminate-Request publishes an
// IPv6 release. The IPv4 leg is the control that the guard is not blanket.
func TestNCPReleaseSkipsUnpublishedFamily(t *testing.T) {
	session, frames, events := newRFC1661Session(LCPStateOpened)
	owner := testHandleFor(session)
	backend, ok := owner.backend.(*fakeBackend)
	if !ok {
		t.Fatalf("session backend = %T, want recorder", owner.backend)
	}
	fixture := &ncpFamilyLifecycle{session: session, frames: frames, events: events, backend: backend}
	t.Cleanup(func() {
		close(session.sessStop)
		fixture.workers.Wait()
		if err := session.stopIPv6Service(); err != nil {
			t.Errorf("fixture IPv6 cleanup: %v", err)
		}
		if err := owner.Close(); err != nil {
			t.Errorf("fixture session cleanup: %v", err)
		}
	})
	session.ncpStarted = true
	session.disableIPCP = false
	session.disableIPv6CP = false
	session.localIPv4 = ipcpTestLocal
	session.peerIPv4 = ipcpTestPeer
	session.localInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 1}
	session.peerInterfaceIDNegotiated = false
	session.setNCPState(AddressFamilyIPv4, LCPStateOpened)
	session.setNCPState(AddressFamilyIPv6, LCPStateOpened)
	fixture.acknowledgeAssignment(t, AddressFamilyIPv4, func() bool {
		return session.onNCPOpened(AddressFamilyIPv4)
	})
	if !session.onNCPOpened(AddressFamilyIPv6) {
		t.Fatal("IPv6CP Opened without a negotiated identifier failed the session")
	}
	fixture.assertNoSessionDown(t)

	if session.handleFrame(lcpFrame(ProtoIPv6CP, LCPTerminateRequest, 31, nil)) {
		t.Fatal("IPv6CP Terminate-Request ended PPP")
	}
	fixture.assertNoSessionDown(t)

	if session.handleFrame(lcpFrame(ProtoIPCP, LCPTerminateRequest, 32, nil)) {
		t.Fatal("IPCP Terminate-Request ended PPP")
	}
	select {
	case event := <-events:
		released, ok := event.(EventSessionIPReleased)
		if !ok {
			t.Fatalf("IPCP termination published %T, want the IPv4 release", event)
		}
		if released.Family != AddressFamilyIPv4 {
			t.Fatalf("released family = %s, want ipv4", released.Family)
		}
	default:
		t.Fatal("IPCP termination of a published assignment published no release")
	}
}

// awaitDualStackSessionUp acknowledges each assignment as the transport
// would, records it by family, and returns them once EventSessionUp arrives.
// It fails on EventSessionDown or when either family published nothing.
func awaitDualStackSessionUp(t *testing.T, td *ncpTestDriver) map[AddressFamily]EventSessionIPAssigned {
	t.Helper()
	assigned := make(map[AddressFamily]EventSessionIPAssigned, 2)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event := <-td.driver.EventsOut():
			switch ev := event.(type) {
			case EventSessionIPAssigned:
				ev.Acknowledge()
				assigned[ev.Family] = ev
			case EventSessionUp:
				for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
					if _, ok := assigned[family]; !ok {
						t.Fatalf("session came up with no %s assignment", family)
					}
				}
				return assigned
			case EventSessionDown:
				t.Fatalf("session ended before coming up: %s", ev.Reason)
			}
		case <-deadline:
			t.Fatal("no EventSessionUp after both NCPs negotiated")
		}
	}
}

// awaitFamilyRelease returns the next EventSessionIPReleased, failing on
// EventSessionDown or any other event in between.
func awaitFamilyRelease(t *testing.T, td *ncpTestDriver) EventSessionIPReleased {
	t.Helper()
	select {
	case event := <-td.driver.EventsOut():
		released, ok := event.(EventSessionIPReleased)
		if !ok {
			t.Fatalf("event after the Terminate-Ack = %T, want EventSessionIPReleased", event)
		}
		return released
	case <-time.After(2 * time.Second):
		t.Fatal("no EventSessionIPReleased after the Terminate-Ack")
	}
	return EventSessionIPReleased{}
}
