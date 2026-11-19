// Design: docs/architecture/l2tp/subscriber-session-model.md -- lifecycle consumers check the admitted Control
// Related: reactor_kernel.go -- handlePPPEvent, pppSessionLocked, refusePPPEvent

package l2tp

import (
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestLatePPPEventIsRefusedOnAReusedSessionID proves AC-1: a PPP event the
// driver produced for a session that then ended cannot touch the new session
// that drew the same session ID, publishes nothing and logs one debug line
// naming the refused event, while the new session's own events still apply.
//
// Method: the real ICRQ allocator draws SID 7 twice on tunnel 17 (only its
// two-byte draw is forced). The first row is admitted to a real PPP Driver,
// which produces its LCP-up and its IPCP assignment; both are held, as events
// queued in the driver's channel are. The first row is then removed through
// the captured teardown, the replacement row is allocated on SID 7 and
// admitted to its own Driver, and the held events reach handlePPPEvent.
// MUTATION: dropping the birth/attempt/control checks in pppSessionLocked
// writes the stale address and user name onto the replacement and publishes
// a session-ip-assigned for it; dropping refusePPPEvent leaves no debug line.
func TestLatePPPEventIsRefusedOnAReusedSessionID(t *testing.T) {
	captureBirthSIDDraw(t)
	logs := &lockedBuffer{}
	reactor := newReactorForSnapshot(t)
	reactor.logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	bus := &recordingBus{}
	reactor.eventBus = bus
	insertEstablishedTunnel(t, reactor, 17, 27, netip.MustParseAddrPort("127.0.0.1:1701"))
	tunnel := reactor.tunnelsByLocalID[17]

	captureBirthAdmit(t, reactor, tunnel, 70)
	reactor.tunnelsMu.Lock()
	original := tunnel.sessions[7]
	original.pppInterface = "ppp7"
	reactor.tunnelsMu.Unlock()
	old := newCurrentCoreAdmission(t, reactor, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: 17, sessionID: 7}})
	old.prepareNetwork(t, "stale-user", ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.81")}, [8]byte{})
	staleUp := old.opened
	staleAssignment := old.assign(t, ppp.AddressFamilyIPv4)
	require.Equal(t, "stale-user", staleAssignment.Username)

	require.True(t, reactor.teardownCapturedSession(tunnel, original, nil, cdnResultAdministrative, l2tpevents.TerminateCauseAdminReset))
	// RFC 2661 Section 5.8: the synthetic peer acknowledges the ICRP and the
	// CDN through the real ZLB receive path before it requests its next call.
	ack := tunnel.engine.OnReceive(MessageHeader{IsControl: true, Nr: tunnel.engine.nextSendSeq}, nil, reactor.params.Clock())
	require.Equal(t, ClassZLB, ack.Class)
	captureBirthAdmit(t, reactor, tunnel, 71)
	reactor.tunnelsMu.Lock()
	replacement := tunnel.sessions[7]
	replacement.pppInterface = "ppp7"
	reactor.tunnelsMu.Unlock()
	require.NotSame(t, original, replacement)
	require.NotEqual(t, original.Birth(), replacement.Birth())
	fresh := newCurrentCoreAdmission(t, reactor, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: 17, sessionID: 7}})
	published := len(bus.events())

	reactor.handlePPPEvent(staleUp)
	reactor.handlePPPEvent(staleAssignment)

	reactor.tunnelsMu.Lock()
	require.Same(t, replacement, tunnel.sessions[7], "a stale event must leave the replacement installed")
	require.False(t, replacement.assignedAddr.IsValid(), "a stale assignment must not write its address")
	require.Empty(t, replacement.username, "a stale assignment must not write its user name")
	require.Zero(t, replacement.negotiatedMRU, "a stale LCP-up must not write its negotiated MRU")
	require.Equal(t, ppp.AuthMethodNone, replacement.negotiatedAuthMethod)
	require.Nil(t, replacement.timeoutLifetime, "a stale assignment must not open a network lifetime")
	reactor.tunnelsMu.Unlock()
	require.Len(t, bus.events(), published, "a stale event must publish nothing")
	refused := latePPPRefusals(logs.String())
	require.Len(t, refused, 2, "each refused event logs exactly one line: %v", refused)
	for index, event := range []string{"lcp-up", "ip-assigned"} {
		require.Contains(t, refused[index], "level=DEBUG")
		require.Contains(t, refused[index], "event="+event)
		require.Contains(t, refused[index], "tunnel-id=17")
		require.Contains(t, refused[index], "session-id=7")
		require.Contains(t, refused[index], "another birth or subscriber ID")
	}

	reactor.handlePPPEvent(fresh.opened)
	fresh.prepareNetwork(t, "fresh-user", ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.82")}, [8]byte{})
	reactor.handlePPPEvent(fresh.assign(t, ppp.AddressFamilyIPv4))

	reactor.tunnelsMu.Lock()
	require.Equal(t, netip.MustParseAddr("192.0.2.82"), replacement.assignedAddr, "the replacement's own assignment applies")
	require.Equal(t, "fresh-user", replacement.username)
	require.NotZero(t, fresh.opened.NegotiatedMRU)
	require.Equal(t, fresh.opened.NegotiatedMRU, replacement.negotiatedMRU, "the replacement's own LCP-up applies")
	require.NotNil(t, replacement.timeoutLifetime)
	reactor.tunnelsMu.Unlock()
	var assigned []*l2tpevents.SessionIPAssignedPayload
	for _, emitted := range bus.events()[published:] {
		if emitted.eventType != l2tpevents.SessionIPAssignedEvent {
			continue
		}
		payload, ok := emitted.payload.(*l2tpevents.SessionIPAssignedPayload)
		require.True(t, ok)
		assigned = append(assigned, payload)
	}
	require.Len(t, assigned, 1, "only the replacement's own assignment is published")
	require.Equal(t, replacement.Birth(), assigned[0].Birth)
	require.Equal(t, "192.0.2.82", assigned[0].PeerAddr)
	require.Equal(t, "fresh-user", assigned[0].Username)
	require.Len(t, latePPPRefusals(logs.String()), 2, "the replacement's own events are not refused")
}

// latePPPRefusals returns the log lines refusePPPEvent wrote, in order.
func latePPPRefusals(output string) []string {
	var refused []string
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "PPP event refused") {
			refused = append(refused, line)
		}
	}
	return refused
}
