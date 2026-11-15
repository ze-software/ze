// Design: docs/architecture/route-selection.md -- ordinary selected-route dissemination.
// Related: rfc4271_selected_advertisement_test.go -- live ordinary selection controls.
// Related: docs/architecture/rib-transition.md -- the default advertiser remains unimplemented.
// RFC naming: untagged -- these defect probes receive no conformance credit while the ordinary advertiser is absent.
package reactor

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestRFC4271OrdinarySelectedMEDChangeReachesPeer changes only one candidate's
// MED after both same-neighbor-AS paths are retained. The initial selected
// advertisement must arrive before the replacement is judged on the socket.
// VALIDATES: a MED-driven best change reaches an ordinary recipient, rather
// than a route-server per-recipient election or a LOCAL_PREF change.
// RFC 4271 Section 9.1.2.2: "Remove from consideration routes with less-preferred
// MULTI_EXIT_DISC attributes." Section 9.2: "All newly installed routes and all
// newly unfeasible routes for which there is no replacement route SHALL be
// advertised to its peers by means of an UPDATE message."
func TestRFC4271OrdinarySelectedMEDChangeReachesPeer(t *testing.T) {
	peers := ordinarySelectedPeers(t, false)
	a, b, recipient := peers[0], peers[1], peers[2]
	prefix := netip.MustParsePrefix("198.19.250.0/24")
	ownershipLiveSend(t, a, nil, ordinaryBoundaryMEDAttrs("198.18.249.1", 20), prefix)
	ownershipLiveSend(t, b, nil, ordinaryBoundaryMEDAttrs("198.18.249.2", 30), prefix)
	ordinarySelectedStored(t, a, prefix)
	ordinarySelectedStored(t, b, prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.1")
	ordinarySelectedWire(t, recipient, prefix, "198.18.249.1")
	ownershipLiveSend(t, b, nil, ordinaryBoundaryMEDAttrs("198.18.249.2", 10), prefix)
	ordinarySelectedBest(t, prefix, "198.18.249.2")
	view := ordinarySelectedWire(t, recipient, prefix, "198.18.249.2")
	require.Len(t, view.announcements[prefix], 2, "the MED-only winner change must replace the prior advertisement")
	ownershipLiveWithdrawals(t, view, prefix, 0)
	ownershipLiveEstablished(t, peers)
}

// TestRFC4271OrdinarySelectedIBGPSplitHorizon pairs the forbidden internal
// route with an external-source route that must reach the same internal
// recipient. The subject must also reach the permitted external recipient.
// VALIDATES: the ordinary advertiser retains iBGP split horizon without
// dropping every selected route or disabling the internal destination.
// RFC 4271 Section 9.2: "When a BGP speaker receives an UPDATE message from an
// internal peer, the receiving BGP speaker SHALL NOT re-distribute the routing
// information contained in that UPDATE message to other internal peers
// (unless the speaker acts as a BGP Route Reflector [RFC2796])."
func TestRFC4271OrdinarySelectedIBGPSplitHorizon(t *testing.T) {
	peers := ordinaryBoundaryPeers(t,
		acceptingLiveSettings(t, "192.0.2.231", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.232", 65000, 65002),
		acceptingLiveSettings(t, "192.0.2.233", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.234", 65000, 65004),
	)
	internalSource, externalSource := peers[0], peers[1]
	internalRecipient, externalRecipient := peers[2], peers[3]
	control := netip.MustParsePrefix("198.19.251.0/24")
	subject := netip.MustParsePrefix("198.19.252.0/24")
	marker := netip.MustParsePrefix("198.19.253.0/24")
	externalAttrs := synthesizedNextHopAttrs(netip.MustParseAddr("198.18.249.2"), 65002)
	ownershipLiveSend(t, externalSource, nil, externalAttrs, control)
	ordinarySelectedStored(t, externalSource, control)
	ordinarySelectedBest(t, control, "198.18.249.2")
	ordinarySelectedWire(t, internalRecipient, control, "198.18.249.2")
	ownershipLiveSend(t, internalSource, nil, ordinarySelectedAttrs("198.18.249.1", 100), subject)
	ordinarySelectedStored(t, internalSource, subject)
	ordinarySelectedBest(t, subject, "198.18.249.1")
	ordinarySelectedWire(t, externalRecipient, subject, "198.18.249.1")
	ordinaryBoundaryDrain(t, internalSource)
	ownershipLiveSend(t, externalSource, nil, externalAttrs, marker)
	ordinarySelectedBest(t, marker, "198.18.249.2")
	ordinarySelectedWire(t, internalRecipient, marker, "198.18.249.2")
	ordinaryBoundaryDrain(t, externalSource)
	ordinaryBoundaryCapture(t, internalRecipient)
	view := ownershipLiveRead(t, internalRecipient)
	require.Empty(t, view.announcements[subject], "an ordinary internal recipient must never receive the iBGP-learned subject")
	ownershipLiveAbsent(t, view, subject)
	ownershipLiveWithdrawals(t, view, subject, 0)
	require.Len(t, view.announcements[control], 1, "the external-source control must remain usable at this internal recipient")
	ownershipLiveEstablished(t, peers)
}

// TestRFC8212OrdinarySelectedAbsentExportPolicyWithholdsRoute requires the
// same selected route at an explicitly accepting external destination before
// inspecting the destination with no export policy. A source marker and joined
// consumer/writer work bound the observation; an entirely inert advertiser
// cannot pass the positive control.
// VALIDATES: ordinary selected-route export keeps RFC 8212's default rejection.
// RFC 8212 Section 3: "Routes SHALL NOT be added to an Adj-RIB-Out associated
// with an EBGP peer if no explicit Export Policy has been applied."
func TestRFC8212OrdinarySelectedAbsentExportPolicyWithholdsRoute(t *testing.T) {
	sourceSettings := acceptingLiveSettings(t, "192.0.2.235", 65000, 65000)
	acceptedSettings := acceptingLiveSettings(t, "192.0.2.236", 65000, 65003)
	withheldSettings := acceptingLiveSettings(t, "192.0.2.237", 65000, 65004)
	withheldSettings.ExportPolicy = filterapi.Policy{}
	require.True(t, acceptedSettings.ExportPolicy.Configured())
	require.False(t, withheldSettings.ExportPolicy.Configured())
	peers := ordinaryBoundaryPeers(t, sourceSettings, acceptedSettings, withheldSettings)
	source, accepted, withheld := peers[0], peers[1], peers[2]
	subject := netip.MustParsePrefix("198.19.254.0/24")
	marker := netip.MustParsePrefix("198.19.255.0/24")
	attrs := ordinarySelectedAttrs("198.18.249.1", 100)
	ownershipLiveSend(t, source, nil, attrs, subject)
	ordinarySelectedStored(t, source, subject)
	ordinarySelectedBest(t, subject, "198.18.249.1")
	ordinarySelectedWire(t, accepted, subject, "198.18.249.1")
	ownershipLiveSend(t, source, nil, attrs, marker)
	ordinarySelectedBest(t, marker, "198.18.249.1")
	ordinarySelectedWire(t, accepted, marker, "198.18.249.1")
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, withheld)
	view := ownershipLiveRead(t, withheld)
	for _, prefix := range []netip.Prefix{subject, marker} {
		require.Empty(t, view.announcements[prefix], "absent export policy must refuse %s", prefix)
		ownershipLiveAbsent(t, view, prefix)
		ownershipLiveWithdrawals(t, view, prefix, 0)
	}
	ownershipLiveEstablished(t, peers)
}

// ordinaryBoundaryMEDAttrs uses equal LOCAL_PREF, origin and AS_PATH in both
// candidates. AS_PATH begins with the same 65010, so MED is comparable even
// though the two distinct TCP sources are iBGP peers.
func ordinaryBoundaryMEDAttrs(hop string, med uint32) []byte {
	attrs := ordinarySelectedAttrs(hop, 100)
	return append(attrs, makeAttr(0x80, byte(attribute.AttrMED), binary.BigEndian.AppendUint32(nil, med))...)
}

// ordinaryBoundaryPeers varies the source/destination session classes while
// using the existing real SDK and socket fixture, without RS/RR. Terminal
// interior host routes resolve both possible source next hops independently.
func ordinaryBoundaryPeers(t *testing.T, settings ...*PeerSettings) []*lowLivePeer {
	t.Helper()
	peers := lowLiveStart(t, lowLiveOptions{withoutRS: true}, settings...)
	loc := locrib.Default()
	require.NotNil(t, loc)
	protocol := redistevents.RegisterProtocol("ordinary-selected-boundary-test")
	for _, address := range []string{"198.18.249.1", "198.18.249.2"} {
		host := netip.PrefixFrom(netip.MustParseAddr(address), 32)
		loc.Insert(family.IPv4Unicast, host, locrib.Path{Source: protocol, Metric: 7})
		t.Cleanup(func() { loc.Remove(family.IPv4Unicast, host, protocol, 0) })
		require.True(t, igpcost.Resolve(loc, host.Addr()).Resolved)
	}
	live := make([]*lowLivePeer, 0, len(peers))
	for i, peer := range peers {
		require.False(t, peer.Settings().RSClient)
		require.False(t, peer.Settings().RouteReflectorClient)
		live = append(live, lowLiveConnect(t, peer, 0x0a00f101+uint32(i)))
	}
	return live
}

// ordinaryBoundaryDrain joins ingress publication, actual SDK consumers and
// the shared forward writer before inspecting a forbidden recipient. The
// preceding socket controls are still required; draining an idle writer alone
// would say nothing about whether ordinary advertisement works. A caller
// inspecting forbidden output MUST then join its capture with
// ordinaryBoundaryCapture: writer completion alone does not join readFrames.
func ordinaryBoundaryDrain(t *testing.T, source *lowLivePeer) {
	t.Helper()
	before := source.peer.Stats().KeepalivesReceived
	source.send(t, message.PackTo(message.NewKeepalive(), nil))
	lowEventually(t, func() bool { return source.peer.Stats().KeepalivesReceived > before }, "ordinary source ingress marker")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, source.peer.receiveCut().wait(ctx))
	for _, name := range []string{"bgp-rib", "bgp-adj-rib-in"} {
		process := source.peer.reactor.api.ProcessManager().GetProcess(name)
		require.NotNil(t, process)
		require.NoError(t, process.DrainEventsApplied(ctx))
	}
	forwardSocketBarrier(t, source.peer.reactor)
}

// ordinaryBoundaryCapture MUST follow ordinaryBoundaryDrain before an absence
// snapshot. The KEEPALIVE uses the destination's real session writer after
// queued UPDATE writes; its observed frame means readFrames appended those
// earlier UPDATEs. Merely returning from net.Pipe.Write would leave a gap
// between the reader copying a body and appending that frame to its history.
func ordinaryBoundaryCapture(t *testing.T, destination *lowLivePeer) {
	t.Helper()
	before := ordinaryBoundaryKeepalives(destination)
	session := destination.peer.currentSession()
	require.NotNil(t, session)
	connection := session.Conn()
	require.NotNil(t, connection)
	require.NoError(t, session.sendKeepalive(connection))
	lowEventually(t, func() bool {
		return ordinaryBoundaryKeepalives(destination) > before
	}, "forbidden recipient captured its post-drain KEEPALIVE")
}

// ordinaryBoundaryKeepalives reads the bounded history under its capture lock.
func ordinaryBoundaryKeepalives(destination *lowLivePeer) int {
	destination.mu.Lock()
	defer destination.mu.Unlock()
	count := 0
	for _, frame := range destination.frames {
		if frame[18] == byte(msgtype.TypeKEEPALIVE) {
			count++
		}
	}
	return count
}
