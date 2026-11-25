// Design: docs/guide/config-reload.md -- a failed reload restores live policy.
// Related: policy_selected_reload_test.go -- same-session selected restoration.
package reactor

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestPolicySelectedRemovalRollbackPreservesAPIOrigin follows the actual
// selected RIB and recipient socket through an applied policy removal and the
// journal undo after a later transaction step fails. The injected step supplies
// only the failure: route admission, retirement and restoration all use their
// production consumers. The API route must survive without any resend.
// MUTATION: omit the journaled settings undo or retire explicit origins during
// learned-only reconciliation; the final socket history then loses a route or
// records an extra API announcement/withdrawal.
func TestPolicySelectedRemovalRollbackPreservesAPIOrigin(t *testing.T) {
	sourceSettings := acceptingLiveSettings(t, "192.0.2.185", 65000, 65010)
	destinationSettings := acceptingLiveSettings(t, "192.0.2.186", 65000, 65000)
	// The borrowed fixture skips the daemon's configured-owner rebuild.
	// Resolve the same grants before creating peers so the API offers its
	// origin to the real RIB by the same owner the running session confirms.
	const ribName = "bgp-rib"
	lowLiveAttachProcess(t, sourceSettings, ribName, "update state refresh")
	lowLiveAttachProcess(t, destinationSettings, ribName, "update state refresh")
	rib := registry.Lookup(ribName)
	require.NotNil(t, rib)
	configured := DisseminationPeersFromSettings([]*PeerSettings{sourceSettings, destinationSettings})
	graph := buildTestDisseminationGraph(t, configured)
	owner, err := graph.ResolveDisseminationOwner(configured[1].Addr, SelectedDisseminationRole,
		[]pluginserver.DisseminationProcess{{Name: ribName, Claims: rib.Claims, Declarations: rib.Dissemination}})
	require.NoError(t, err)
	require.Equal(t, ribName, owner.Name())
	destinationSettings.DisseminationOwner = owner
	peers := ordinaryBoundaryPeers(t, sourceSettings, destinationSettings)
	source, destination := peers[0], peers[1]
	session := destination.peer.currentSession()
	require.NotNil(t, session)
	require.True(t, destination.peer.ordinarySelectedExport(), "the recipient must use ordinary selected output")
	require.Equal(t, owner, destination.peer.selectedOwnerSnapshot().owner)
	api := &reactorAPIAdapter{r: destination.peer.reactor}
	learned := netip.MustParsePrefix("198.19.185.0/24")
	local := netip.MustParsePrefix("198.19.186.0/24")

	ownershipLiveSend(t, source, nil, ordinarySelectedAttrs("198.18.249.1", 100), learned)
	ordinarySelectedStored(t, source, learned)
	ordinarySelectedBest(t, learned, "198.18.249.1")
	ordinarySelectedWire(t, destination, learned, "198.18.249.1")
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, local, 0, false)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("198.18.249.2")),
		Wire:    attribute.NewAttributesWire([]byte{0x40, 1, 1, 0, 0x40, 2, 0}, bgpctx.APIContextID),
	}
	result, err := api.AnnounceNLRIBatchResult(t.Context(), selector.ParseDefault(destination.peer.addrString), batch, plugin.OperatorSender())
	require.NoError(t, err)
	require.Equal(t, uint32(1), result.Announced)
	require.Len(t, result.Admissions, 1, "the actual API origin must enter the configured ordinary origin domain")
	ordinarySelectedWire(t, destination, local, "198.18.249.2")
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, destination)
	before := ownershipLiveRead(t, destination)
	require.Len(t, before.announcements[learned], 1)
	require.Len(t, before.announcements[local], 1)
	ownershipLiveWithdrawals(t, before, learned, 0)
	ownershipLiveWithdrawals(t, before, local, 0)

	next := destination.peer.settingsSnapshot()
	next.ExportPolicy = filterapi.Policy{}
	journal := &internalJournal{}
	require.NoError(t, api.swapPeerSettingsJournaled([]peerSettingsSwap{{
		key: destination.peer.Settings().PeerKey(), next: next, apply: hotSwappableSettings,
	}}, journal))
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, destination)
	removed := ownershipLiveRead(t, destination)
	ownershipLiveAbsent(t, removed, learned)
	ownershipLiveWithdrawals(t, removed, learned, 1)
	ownershipLiveAnnouncements(t, removed, learned, before.routes[learned])
	ownershipLiveRoute(t, removed, local, before.routes[local])
	ownershipLiveAnnouncements(t, removed, local, before.routes[local])
	ownershipLiveWithdrawals(t, removed, local, 0)

	failure := errors.New("a later reload participant refused its apply")
	err = journal.Record(func() error { return failure }, func() error { return nil })
	require.ErrorIs(t, err, failure)
	require.Empty(t, journal.Rollback(), "the real settings undo must finish restoring selected output")
	ordinarySelectedWire(t, destination, learned, "198.18.249.1")
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, destination)
	restored := ownershipLiveRead(t, destination)
	ownershipLiveRoute(t, restored, learned, before.routes[learned])
	ownershipLiveAnnouncements(t, restored, learned, before.routes[learned], before.routes[learned])
	ownershipLiveWithdrawals(t, restored, learned, 1)
	ownershipLiveRoute(t, restored, local, before.routes[local])
	ownershipLiveAnnouncements(t, restored, local, before.routes[local])
	ownershipLiveWithdrawals(t, restored, local, 0)
	require.Same(t, session, destination.peer.currentSession(), "rollback must keep the original recipient session")
	ownershipLiveEstablished(t, peers)
}
