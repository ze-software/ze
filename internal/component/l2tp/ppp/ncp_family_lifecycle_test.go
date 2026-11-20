// Design: docs/research/l2tpv2-ze-integration.md -- independent NCP resource lifetimes
// Related: ncp.go -- handleNCPPacket, performNCPAction and onNCPOpened
// RFC: rfc/short/rfc1661.md -- Sections 3.6, 3.7 and 4.4

package ppp

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// A peer's new Configure-Request leaves that NCP's Opened state. Drive the
// actual frame dispatcher and require only that family's address to leave.
// RFC 1661 Section 4.4: "This action indicates to the upper layers that the
// automaton is leaving the Opened state."
func TestNCPRenegotiationWithdrawsOnlyItsFamily(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			fixture := newNCPFamilyLifecycle(t)
			previousService := fixture.session.ipv6Svc
			if fixture.session.handleFrame(fixture.configureRequest(family)) {
				t.Fatal("one family's Configure-Request ended the PPP session")
			}
			fixture.expectFamilyRelease(t, family)
			if got := fixture.session.ncpState(family); got != LCPStateAckSent {
				t.Fatalf("restarting NCP state = %s, want Ack-Sent", got)
			}
			if got := fixture.session.ncpState(otherNCPFamily(family)); got != LCPStateOpened {
				t.Fatalf("independent NCP state = %s, want Opened", got)
			}
			removes := fixture.backend.AddrRemoveCalls()
			if len(removes) != 1 {
				t.Fatalf("address withdrawals = %+v, want only the restarting family", removes)
			}
			if removes[0].cidr != fixture.localPrefix(family).String() {
				t.Fatalf("withdrawn address = %s, want %s", removes[0].cidr, fixture.localPrefix(family))
			}
			if family == AddressFamilyIPv4 {
				if fixture.session.ipv6Svc != previousService {
					t.Fatal("IPCP restart stopped the independent IPv6 service")
				}
			} else if fixture.session.ipv6Svc != nil {
				t.Fatal("IPv6CP restart retained the old IPv6 service")
			}
			fixture.assertNoSessionDown(t)
		})
	}
}

// A fresh peer Ack completes the restarted NCP on the connected transport.
// Require its pair and IPv6 service to return without repeating the other pair.
func TestNCPReopenRestoresOnlyItsFamily(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			fixture := newNCPFamilyLifecycle(t)
			before := len(fixture.backend.P2PCalls())
			if fixture.session.handleFrame(fixture.configureRequest(family)) {
				t.Fatal("Configure-Request ended the PPP session")
			}
			fixture.expectFamilyRelease(t, family)
			request := fixture.lastConfigureRequest(t, family)
			fixture.acknowledgeAssignment(t, family, func() bool {
				return !fixture.session.handleFrame(lcpFrame(ncpProto(family), LCPConfigureAck,
					request.Identifier, request.Data))
			})
			if got := fixture.session.ncpState(family); got != LCPStateOpened {
				t.Fatalf("reopened NCP state = %s, want Opened", got)
			}
			writes := fixture.backend.P2PCalls()
			if len(writes) != before+1 {
				t.Fatalf("address writes = %+v, want exactly one reopened pair", writes)
			}
			if writes[len(writes)-1].local != fixture.localPrefix(family).String() {
				t.Fatalf("reopened local address = %s, want %s", writes[len(writes)-1].local,
					fixture.localPrefix(family))
			}
			if family == AddressFamilyIPv6 && len(fixture.carriers) != 2 {
				t.Fatalf("IPv6 carriers created = %d, want a fresh service after restart", len(fixture.carriers))
			}
			if got := fixture.session.ncpState(otherNCPFamily(family)); got != LCPStateOpened {
				t.Fatalf("independent NCP state = %s, want Opened", got)
			}
			fixture.assertNoSessionDown(t)
		})
	}
}

// A valid Nak changes the negotiation proposal before the FSM runs TLD.
// Cleanup must withdraw the address that was installed, never the new proposal.
func TestNCPNakWithdrawsInstalledIPv4Pair(t *testing.T) {
	fixture := newNCPFamilyLifecycle(t)
	installed := fixture.localPrefix(AddressFamilyIPv4)
	if !fixture.session.sendNCPConfigureRequest(AddressFamilyIPv4) {
		t.Fatal("could not record the current IPCP request")
	}
	request := fixture.lastConfigureRequest(t, AddressFamilyIPv4)
	proposal := netip.MustParseAddr("10.0.0.9")
	if fixture.session.handleFrame(lcpFrame(ProtoIPCP, LCPConfigureNak, request.Identifier,
		ipcpOpt(IPCPOptIPAddress, proposal))) {
		t.Fatal("a valid IPCP Nak ended the PPP session")
	}
	fixture.expectFamilyRelease(t, AddressFamilyIPv4)
	if fixture.session.localIPv4 != proposal {
		t.Fatalf("new negotiation proposal = %s, want %s", fixture.session.localIPv4, proposal)
	}
	removes := fixture.backend.AddrRemoveCalls()
	if len(removes) != 1 {
		t.Fatalf("address withdrawals = %+v, want the installed pair", removes)
	}
	if removes[0].cidr != installed.String() {
		t.Fatalf("withdrawn address = %s, want old installed address %s", removes[0].cidr, installed)
	}
	fixture.assertNoSessionDown(t)
}

// A failed withdrawal cannot be treated as a reusable NCP lifetime. Require a
// final failure, no replacement request and a later retry of the retained pair.
func TestNCPWithdrawalFailurePreservesCleanupOwner(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			fixture := newNCPFamilyLifecycle(t)
			failure := errors.New("NCP address withdrawal refused")
			retained := fixture.localPrefix(family).String()
			fixture.backend.mu.Lock()
			fixture.backend.removeAddrErr = failure
			fixture.backend.mu.Unlock()
			before := fixture.frames.count()
			if !fixture.session.handleFrame(fixture.configureRequest(family)) {
				t.Fatal("failed NCP withdrawal allowed replacement negotiation")
			}
			if fixture.frames.count() != before {
				t.Fatal("failed withdrawal sent replacement negotiation frames")
			}
			select {
			case event := <-fixture.events:
				ended, ok := event.(EventSessionDown)
				if !ok {
					t.Fatalf("failure event = %T, want SessionDown", event)
				}
				if !strings.Contains(ended.Reason, failure.Error()) {
					t.Fatalf("failure reason %q lost the withdrawal cause", ended.Reason)
				}
			default:
				t.Fatal("failed withdrawal omitted the final session event")
			}
			beforeRetry := 0
			for _, removal := range fixture.backend.AddrRemoveCalls() {
				if removal.cidr == retained {
					beforeRetry++
				}
			}
			if beforeRetry == 0 {
				t.Fatal("the failed withdrawal never attempted its owned address")
			}
			fixture.backend.mu.Lock()
			fixture.backend.removeAddrErr = nil
			fixture.backend.mu.Unlock()
			if err := fixture.session.teardownNCPResources(); err != nil {
				t.Fatalf("retry of retained NCP cleanup: %v", err)
			}
			removes := fixture.backend.AddrRemoveCalls()
			afterRetry := 0
			for _, removal := range removes {
				if removal.cidr == retained {
					afterRetry++
				}
			}
			if afterRetry != beforeRetry+1 {
				t.Fatalf("withdrawals of retained address %s = %d, want %d: %+v",
					retained, afterRetry, beforeRetry+1, removes)
			}
		})
	}
}

// RFC 1661 Section 3.7 says closing one NCP is not enough to end PPP. Drive
// both Terminate packets and require the independently opened family to live.
func TestNCPClosingOneFamilyPreservesOtherFamily(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			fixture := newNCPFamilyLifecycle(t)
			if fixture.session.handleFrame(lcpFrame(ncpProto(family), LCPTerminateRequest, 31, nil)) {
				t.Fatal("one NCP's Terminate-Request ended PPP")
			}
			fixture.expectFamilyRelease(t, family)
			if fixture.session.handleFrame(lcpFrame(ncpProto(family), LCPTerminateAck, 31, nil)) {
				t.Fatal("one NCP's Stopped state ended the independent family")
			}
			if got := fixture.session.ncpState(otherNCPFamily(family)); got != LCPStateOpened {
				t.Fatalf("independent NCP state = %s, want Opened", got)
			}
			fixture.assertNoSessionDown(t)
		})
	}
}

type ncpFamilyLifecycle struct {
	session  *pppSession
	frames   *frameRecorder
	events   chan Event
	backend  *fakeBackend
	carriers []*serviceTestCarrier
	workers  sync.WaitGroup
}

// The fixture installs both pairs through the existing NCP/service producers,
// acknowledges their accounting barriers, then models the connected channel.
func newNCPFamilyLifecycle(t *testing.T) *ncpFamilyLifecycle {
	t.Helper()
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
	owner.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		carrier := newServiceCarrier()
		fixture.carriers = append(fixture.carriers, carrier)
		return carrier, nil
	}
	session.ncpStarted = true
	session.disableIPCP = false
	session.disableIPv6CP = false
	session.localIPv4 = ipcpTestLocal
	session.peerIPv4 = ipcpTestPeer
	session.localInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 1}
	session.peerInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 2}
	session.peerInterfaceIDNegotiated = true
	session.setNCPState(AddressFamilyIPv4, LCPStateOpened)
	session.setNCPState(AddressFamilyIPv6, LCPStateOpened)
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		fixture.acknowledgeAssignment(t, family, func() bool { return session.onNCPOpened(family) })
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	session.channelConnected = true
	if err := session.afterLCPOpenIPv6Service(); err != nil {
		t.Fatalf("initial IPv6 service failed the session: %v", err)
	}
	if session.ipv6Svc == nil {
		t.Fatal("initial IPv6 service was not created")
	}
	return fixture
}

func (fixture *ncpFamilyLifecycle) localPrefix(family AddressFamily) netip.Prefix {
	if family == AddressFamilyIPv4 {
		return netip.PrefixFrom(fixture.session.localIPv4, 32)
	}
	if family == AddressFamilyIPv6 {
		return netip.PrefixFrom(linkLocalFromInterfaceID(fixture.session.localInterfaceID), linkLocalPrefixBits)
	}
	panic("BUG: unknown NCP fixture family")
}

func (fixture *ncpFamilyLifecycle) configureRequest(family AddressFamily) []byte {
	if family == AddressFamilyIPv4 {
		return lcpFrame(ProtoIPCP, LCPConfigureRequest, 29, ipcpOpt(IPCPOptIPAddress, fixture.session.peerIPv4))
	}
	if family == AddressFamilyIPv6 {
		option := [10]byte{IPv6CPOptInterfaceID, 10}
		copy(option[2:], fixture.session.peerInterfaceID[:])
		return lcpFrame(ProtoIPv6CP, LCPConfigureRequest, 29, option[:])
	}
	panic("BUG: unknown NCP fixture family")
}

func (fixture *ncpFamilyLifecycle) lastConfigureRequest(t *testing.T, family AddressFamily) LCPPacket {
	t.Helper()
	frames := decodeFrames(t, fixture.frames)
	for _, frame := range slices.Backward(frames) {
		if frame.Proto == ncpProto(family) {
			if frame.Pkt.Code == LCPConfigureRequest {
				return frame.Pkt
			}
		}
	}
	t.Fatal("NCP did not send its replacement Configure-Request")
	return LCPPacket{}
}

// One test-owned producer runs until the assignment is acknowledged. Both
// waits have failure bounds; no duration is evidence that publication finished.
func (fixture *ncpFamilyLifecycle) acknowledgeAssignment(t *testing.T, family AddressFamily, produce func() bool) {
	t.Helper()
	done := make(chan bool, 1)
	fixture.workers.Go(func() { done <- produce() })
	select {
	case event := <-fixture.events:
		assigned, ok := event.(EventSessionIPAssigned)
		if !ok {
			t.Fatalf("NCP event = %T, want assignment", event)
		}
		if assigned.Family != family {
			t.Fatalf("assignment family = %s, want %s", assigned.Family, family)
		}
		assigned.Acknowledge()
	case result := <-done:
		t.Fatalf("NCP producer returned %v before assignment", result)
	case <-time.After(time.Second):
		t.Fatal("NCP did not publish its assignment")
	}
	select {
	case success := <-done:
		if !success {
			t.Fatal("NCP producer failed after acknowledgment")
		}
	case <-time.After(time.Second):
		t.Fatal("NCP did not consume its assignment acknowledgment")
	}
}

func (fixture *ncpFamilyLifecycle) assertNoSessionDown(t *testing.T) {
	t.Helper()
	select {
	case event := <-fixture.events:
		t.Fatalf("independent NCP transition published unexpected %T", event)
	default:
	}
}

// expectFamilyRelease requires the next event to be the release of exactly
// this family, for this session and the network lifetime its published
// assignment carried (AC-4; RFC 1661 Section 4.4, tld). It consumes only that
// one event, so a following assertNoSessionDown still proves that nothing
// else, the other family's release included, was published. It MUST be called
// on the test goroutine while no producer worker runs: it reads the session's
// network lifetime, which the fixture's own frames alone change.
func (fixture *ncpFamilyLifecycle) expectFamilyRelease(t *testing.T, family AddressFamily) EventSessionIPReleased {
	t.Helper()
	select {
	case event := <-fixture.events:
		released, ok := event.(EventSessionIPReleased)
		if !ok {
			t.Fatalf("%s leaving Opened published %T, want its release", family, event)
		}
		if released.Family != family {
			t.Fatalf("released family = %s, want %s", released.Family, family)
		}
		if released.TunnelID != fixture.session.tunnelID {
			t.Fatalf("release names tunnel %d, want %d", released.TunnelID, fixture.session.tunnelID)
		}
		if released.SessionID != fixture.session.sessionID {
			t.Fatalf("release names session %d, want %d", released.SessionID, fixture.session.sessionID)
		}
		lifetime, assigned := fixture.session.assignedNetworkLifetime()
		if !assigned {
			t.Fatal("release published with no network lifetime assigned")
		}
		if released.NetworkLifetime != lifetime {
			t.Fatalf("release network lifetime = %+v, want the published assignment's %+v",
				released.NetworkLifetime, lifetime)
		}
		return released
	default:
		t.Fatalf("%s leaving Opened published no release", family)
	}
	return EventSessionIPReleased{}
}

func otherNCPFamily(family AddressFamily) AddressFamily {
	if family == AddressFamilyIPv4 {
		return AddressFamilyIPv6
	}
	if family == AddressFamilyIPv6 {
		return AddressFamilyIPv4
	}
	panic("BUG: unknown NCP fixture family")
}
