// Design: docs/guide/config-reload.md -- same-session selected export restoration.
// Related: rfc4271_selected_boundaries_test.go -- real SDK and socket controls.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// MUTATION: classify final changes with AcceptsAtEnd instead of the received
// route kind; internal restoration sends nothing. Omitting the targeted owner
// event also strands both internal and external retained elected routes.
func TestPolicySelectedFinalReloadRestoresSameSession(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "internal", true: "external"}[external], func(t *testing.T) {
			sourceSettings := acceptingLiveSettings(t, "192.0.2.181", 65000, 65010)
			acceptedSettings := acceptingLiveSettings(t, "192.0.2.182", 65000, 65003)
			remoteAS := uint32(65000)
			if external {
				remoteAS = 65004
			}
			destinationSettings := acceptingLiveSettings(t, "192.0.2.183", 65000, remoteAS)
			destinationSettings.ExportPolicy = filterapi.Policy{}
			local := netip.MustParsePrefix("198.19.181.0/24")
			if !external {
				destinationSettings.StaticRoutes = []StaticRoute{staticRouteAt(local.String(), "198.18.249.2")}
			}
			peers := ordinaryBoundaryPeers(t, sourceSettings, acceptedSettings, destinationSettings)
			source, accepted, destination := peers[0], peers[1], peers[2]
			captured := destination.peer.currentSession()
			subject := netip.MustParsePrefix("198.19.182.0/24")
			ownershipLiveSend(t, source, nil, ordinarySelectedAttrs("198.18.249.1", 100), subject)
			ordinarySelectedStored(t, source, subject)
			ordinarySelectedBest(t, subject, "198.18.249.1")
			ordinarySelectedWire(t, accepted, subject, "198.18.249.1")
			ordinaryBoundaryDrain(t, source)
			ordinaryBoundaryCapture(t, destination)
			before := ownershipLiveRead(t, destination)
			require.Empty(t, before.announcements[subject], "the learned route must first be withheld")
			if !external {
				require.Len(t, before.announcements[local], 1, "absent internal policy must keep explicit local output useful")
			}

			next := destination.peer.settingsSnapshot()
			next.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
			mustApplyHotSwappableSettings(t, destination.peer, next, hotSwappableSettings)
			ordinarySelectedWire(t, destination, subject, "198.18.249.1")
			ordinaryBoundaryCapture(t, destination)
			restored := ownershipLiveRead(t, destination)
			require.Len(t, restored.announcements[subject], 1, "no new source UPDATE or destination connection is needed")
			if !external {
				require.Len(t, restored.announcements[local], 1, "learned-only restoration MUST NOT resend local routes")
				ownershipLiveWithdrawals(t, restored, local, 0)
			}

			next = destination.peer.settingsSnapshot()
			next.ExportPolicy = filterapi.Policy{}
			mustApplyHotSwappableSettings(t, destination.peer, next, hotSwappableSettings)
			ordinaryBoundaryCapture(t, destination)
			denied := ownershipLiveRead(t, destination)
			ownershipLiveAbsent(t, denied, subject)
			ownershipLiveWithdrawals(t, denied, subject, 1)
			if !external {
				require.Len(t, denied.announcements[local], 1)
				ownershipLiveWithdrawals(t, denied, local, 0)
				require.Contains(t, denied.routes, local, "learned-only rejection preserves the local survivor")
			}
			require.Same(t, captured, destination.peer.currentSession())
			ownershipLiveEstablished(t, peers)
		})
	}
}

// The actual registered provider opts into C2 OriginIntake and selects a local
// receipt. A learned-only final change must retain that receipt and wire output
// without reissuing its input. Both native typed and JSON socket rails run.
func TestPolicySelectedFinalReloadKeepsOriginIntakeLocal(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginRuntimeSDKFixture(t, typed)
			peer := f.writer.peer
			peer.settings.PeerAS = peer.settings.LocalAS
			peer.settings.ExportPolicy = filterapi.Policy{}
			peer.refreshForwardFacts()
			peer.session.egressFinalAccepts = func(kind filterapi.ExportKind) bool {
				facts := peer.forwardFacts()
				return facts.exportPolicy.AcceptsExportAtEnd(facts.isEBGP, kind)
			}
			f.writer.api.r.bindEvents = bgpserver.NewEventDispatcher(f.host)
			original := f.announce(t)
			before := f.writer.conn.written()
			require.NotEmpty(t, before)
			next := peer.settingsSnapshot()
			next.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
			mustApplyHotSwappableSettings(t, peer, next, hotSwappableSettings)
			require.Equal(t, before, f.writer.conn.written(), "local selected output must neither be resent nor withdrawn")
			require.Empty(t, f.offers, "unchanged original input must not be reissued")
			f.writer.api.r.originReceipts.mu.Lock()
			record := f.writer.api.r.originReceipts.receipts[original.OriginReceipt]
			f.writer.api.r.originReceipts.mu.Unlock()
			require.NotNil(t, record)
			// The live provider remains useful: an unchanged announcement retains
			// one source receipt and the ordinary final writer can still answer.
			require.Equal(t, rpc.OriginIntakeAnnounce, original.Operation)
		})
	}
}

// The selected output records its export kind. Engine-owned retirement must
// preserve a local origin written by the same deciding process, without asking
// a CurrentSelection callback to invent absence or changing its source receipt.
func TestPolicySelectedRetirementUsesRecordedKind(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	local, _ := f.issue(t, selectedOriginBuildInput(), 1)
	answer, err := f.send(t, t.Context(), local)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	learned := f.route(t, 2, "192.0.2.79", 0, false)
	learned.KeyNLRI[3] = 8
	learned.Route.NLRIHex = "18c61208"
	answer, err = f.send(t, t.Context(), learned)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	require.NoError(t, f.peer.retireOrdinaryLearned(t.Context(), f.peer.currentSession()))
	bodies := updateBodies(t, f.conn.written())
	require.Len(t, bodies, 3, "two useful selections followed by the learned withdrawal alone")
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.8.0/24")}, updatePrefixes(t, bodies[2], true))
	require.NoError(t, f.peer.retireOrdinaryLearned(t.Context(), f.peer.currentSession()))
	require.Equal(t, bodies, updateBodies(t, f.conn.written()), "settled retirement cannot duplicate local or learned output")
}
