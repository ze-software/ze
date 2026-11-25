// Design: docs/architecture/rib-transition.md -- scoped origin arbitration.
// Related: policy_reload.go -- reconcile an explicit final policy change.
// Related: rfc4271_selected_boundaries_test.go -- real RIB and socket fixture.
package reactor

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpupdate "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPolicyScopedOriginRejectRetiresOnlyItsRecipient proves AC-15 through
// the real selecting RIB and established recipient sockets. An authenticated
// process replaces the common selection on A only. Denying A's selected route
// must withdraw it while B keeps receiving useful common learned output.
// Rollback and a later explicit accept restore the same admitted origin once.
func TestPolicyScopedOriginRejectRetiresOnlyItsRecipient(t *testing.T) {
	settings := []*PeerSettings{
		acceptingLiveSettings(t, "192.0.2.187", 65000, 65010),
		acceptingLiveSettings(t, "192.0.2.188", 65000, 65000),
		acceptingLiveSettings(t, "192.0.2.189", 65000, 65000),
	}
	prepareScopedOriginOwners(t, settings)
	peers := ordinaryBoundaryPeers(t, settings...)
	source, a, b := peers[0], peers[1], peers[2]
	for _, peer := range []*lowLivePeer{a, b} {
		require.True(t, peer.peer.ordinarySelectedExport())
	}

	const learnedHop = "198.18.249.1"
	const originHop = "198.18.249.2"
	key := netip.MustParsePrefix("198.19.187.0/24")
	marker := netip.MustParsePrefix("198.19.188.0/24")
	direct := netip.MustParsePrefix("198.19.189.0/24")
	attrs := ordinarySelectedAttrs(learnedHop, 100)
	ownershipLiveSend(t, source, nil, attrs, key)
	ordinarySelectedStored(t, source, key)
	ordinarySelectedBest(t, key, learnedHop)
	learnedA := ordinarySelectedWire(t, a, key, learnedHop).routes[key]
	learnedB := ordinarySelectedWire(t, b, key, learnedHop).routes[key]

	announceScopedOrigin(t, a, key, originHop)
	localA := ordinarySelectedWire(t, a, key, originHop).routes[key]
	directA := announceDirectScopeControl(t, a, direct, originHop)
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, b)
	ownershipLiveAnnouncements(t, ownershipLiveRead(t, b), key, learnedB)

	// An explicit rejection applies to the installed local origin. Removing
	// the policy would instead leave that local route eligible on this iBGP peer.
	aSession, bSession := a.peer.currentSession(), b.peer.currentSession()
	next := a.peer.settingsSnapshot()
	accept := next.ExportPolicy
	var err error
	next.ExportPolicy, err = filterapi.NewPolicy(nil, policy.FinalReject)
	require.NoError(t, err)
	api := &reactorAPIAdapter{r: a.peer.reactor}
	journal := &internalJournal{}
	require.NoError(t, api.swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: a.peer.Settings().PeerKey(), next: next, apply: hotSwappableSettings,
	}}, journal))

	ownershipLiveSend(t, source, nil, attrs, marker)
	resultB := ordinarySelectedWire(t, b, marker, learnedHop)
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, a)
	resultA := ownershipLiveRead(t, a)
	ownershipLiveAbsent(t, resultA, key)
	ownershipLiveWithdrawals(t, resultA, key, 1)
	ownershipLiveAnnouncements(t, resultA, key, learnedA, localA)
	ownershipLiveAbsent(t, resultA, direct)
	ownershipLiveWithdrawals(t, resultA, direct, 1)
	ownershipLiveAnnouncements(t, resultA, direct, directA)
	ownershipLiveRoute(t, resultB, key, learnedB)
	ownershipLiveAnnouncements(t, resultB, key, learnedB)
	ownershipLiveWithdrawals(t, resultB, key, 0)

	failure := errors.New("a later reload participant refused its apply")
	err = journal.Record(func() error { return failure }, func() error { return nil })
	require.ErrorIs(t, err, failure)
	require.Empty(t, journal.Rollback(), "rollback must restore the current scoped origin")
	ordinaryBoundaryCapture(t, a)
	restored := ordinarySelectedWire(t, a, key, originHop)
	ownershipLiveRoute(t, restored, key, localA)
	ownershipLiveAnnouncements(t, restored, key, learnedA, localA, localA)
	ownershipLiveWithdrawals(t, restored, key, 1)
	ownershipLiveAbsent(t, restored, direct)
	ownershipLiveAnnouncements(t, restored, direct, directA)
	ownershipLiveWithdrawals(t, restored, direct, 1)

	applyScopedOriginExportPolicy(t, a, next.ExportPolicy)
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, a)
	deniedAgain := ownershipLiveRead(t, a)
	ownershipLiveAbsent(t, deniedAgain, key)
	ownershipLiveAnnouncements(t, deniedAgain, key, learnedA, localA, localA)
	ownershipLiveWithdrawals(t, deniedAgain, key, 2)

	applyScopedOriginExportPolicy(t, a, accept)
	ordinarySelectedWire(t, a, key, originHop)
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, a)
	ordinaryBoundaryCapture(t, b)
	acceptedAgain := ownershipLiveRead(t, a)
	ownershipLiveRoute(t, acceptedAgain, key, localA)
	ownershipLiveAnnouncements(t, acceptedAgain, key, learnedA, localA, localA, localA)
	ownershipLiveWithdrawals(t, acceptedAgain, key, 2)
	ownershipLiveAbsent(t, acceptedAgain, direct)
	ownershipLiveAnnouncements(t, acceptedAgain, direct, directA)
	ownershipLiveWithdrawals(t, acceptedAgain, direct, 1)
	// The raw API sends a one-shot UPDATE. It has to send it again explicitly.
	require.Equal(t, directA, announceDirectScopeControl(t, a, direct, originHop))
	ordinaryBoundaryCapture(t, a)
	reannounced := ownershipLiveRead(t, a)
	ownershipLiveAnnouncements(t, reannounced, direct, directA, directA)
	ownershipLiveWithdrawals(t, reannounced, direct, 1)
	unchanged := ownershipLiveRead(t, b)
	ownershipLiveRoute(t, unchanged, key, learnedB)
	ownershipLiveAnnouncements(t, unchanged, key, learnedB)
	ownershipLiveWithdrawals(t, unchanged, key, 0)
	ownershipLiveRoute(t, unchanged, marker, resultB.routes[marker])
	ownershipLiveAnnouncements(t, unchanged, marker, resultB.routes[marker])
	ownershipLiveWithdrawals(t, unchanged, marker, 0)
	ownershipLiveAbsent(t, unchanged, direct)
	require.Same(t, aSession, a.peer.currentSession())
	require.Same(t, bSession, b.peer.currentSession())
	ownershipLiveEstablished(t, peers)
}

// announceDirectScopeControl sends through the existing raw API and confirms
// that the final writer recorded the route without a selecting owner.
func announceDirectScopeControl(t *testing.T, recipient *lowLivePeer, prefix netip.Prefix, hop string) []byte {
	t.Helper()
	address := prefix.Addr().As4()
	nlri := []byte{24, address[0], address[1], address[2]}
	packet := message.PackTo(&message.Update{
		PathAttributes: ordinarySelectedAttrs(hop, 100), NLRI: nlri,
	}, nil)
	api := &reactorAPIAdapter{r: recipient.peer.reactor}
	session := recipient.peer.currentSession()
	require.NoError(t, api.SendRawMessage(recipient.peer.Settings().Address,
		uint8(msgtype.TypeUPDATE), packet[message.HeaderLen:], plugin.OperatorSender()))
	onWire := ordinarySelectedWire(t, recipient, prefix, hop).routes[prefix]
	var path fwdPathKey
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	require.NoError(t, fwdPathKeyFor(&path, family.IPv4Unicast, 0, nlri, false, scratch[:]))
	table := &recipient.peer.adjOut
	table.mu.Lock()
	entry := table.routes[adjOutKey{path: path}]
	recorded := table.session == session && entry != nil
	selected := entry != nil && entry.selection.owner.Done() != nil
	table.mu.Unlock()
	require.True(t, recorded, "the real writer must record the direct route on the recipient session")
	require.False(t, selected, "this route must exercise the unselected withdrawal path")
	return onWire
}

func applyScopedOriginExportPolicy(t *testing.T, recipient *lowLivePeer, export filterapi.Policy) {
	t.Helper()
	next := recipient.peer.settingsSnapshot()
	next.ExportPolicy = export
	api := &reactorAPIAdapter{r: recipient.peer.reactor}
	require.NoError(t, api.swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: recipient.peer.Settings().PeerKey(), next: next, apply: hotSwappableSettings,
	}}, &internalJournal{}))
}

// prepareScopedOriginOwners resolves the configured grants before the borrowed
// fixture starts. The daemon does this in its configured-peer rebuild.
func prepareScopedOriginOwners(t *testing.T, settings []*PeerSettings) {
	t.Helper()
	const ribName = "bgp-rib"
	for _, peer := range settings {
		lowLiveAttachProcess(t, peer, ribName, "update state refresh")
	}
	rib := registry.Lookup(ribName)
	require.NotNil(t, rib)
	configured := DisseminationPeersFromSettings(settings)
	graph := buildTestDisseminationGraph(t, configured)
	processes := []pluginserver.DisseminationProcess{{Name: ribName, Claims: rib.Claims, Declarations: rib.Dissemination}}
	for i, peer := range settings {
		owner, err := graph.ResolveDisseminationOwner(configured[i].Addr, SelectedDisseminationRole, processes)
		require.NoError(t, err)
		require.Equal(t, ribName, owner.Name())
		peer.DisseminationOwner = owner
	}
}

func announceScopedOrigin(t *testing.T, destination *lowLivePeer, key netip.Prefix, hop string) {
	t.Helper()
	const producer = "bgp-adj-rib-in"
	server := destination.peer.reactor.api
	process := server.ProcessManager().GetProcess(producer)
	require.NotNil(t, process)
	// The fixture supplies this process's real grant and live process token.
	// Update-route parsing and the production command handler issue the origin.
	parsed, err := bgpupdate.ParseUpdateText([]string{"nhop", hop, "nlri", "add", key.String()})
	require.NoError(t, err)
	response, err := bgpupdate.DispatchNLRIGroups(&pluginserver.CommandContext{
		Server: server, Process: process, Sender: plugin.ProcessSender(producer),
		Peer: destination.peer.addrString, RequestContext: t.Context(),
	}, parsed.Groups)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, plugin.StatusDone, response.Status, "%+v", response)
	result, ok := response.Data.(*plugin.RouteResult)
	require.True(t, ok, "update-route must return selected origin admission")
	require.Equal(t, uint32(1), result.Announced)
	require.Len(t, result.Admissions, 1)
	require.Equal(t, rpc.RouteAdmissionSelected, result.Admissions[0].Disposition)
}
