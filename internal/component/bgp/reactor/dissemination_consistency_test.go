// Design: docs/architecture/api/architecture.md -- immutable dissemination authority.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestDisseminationSnapshotSurvivesFactsRefresh proves an unrelated forwarding
// refresh cannot revoke queued selected work for the same owner and connection.
func TestDisseminationSnapshotSurvivesFactsRefresh(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	before := f.peer.selectedOwnerSnapshot()
	require.NotNil(t, before)
	require.NoError(t, before.err)
	f.peer.refreshForwardFacts()
	require.Same(t, before, f.peer.selectedOwnerSnapshot())
}

// TestDisseminationSnapshotChangesWithSession proves retained authority cannot
// authorize a replacement connection, even when its configured grants match.
func TestDisseminationSnapshotChangesWithSession(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	before := f.peer.selectedOwnerSnapshot()
	f.peer.mu.Lock()
	f.peer.session = &Session{}
	f.peer.mu.Unlock()
	f.peer.refreshForwardFacts()
	require.NotSame(t, before, f.peer.selectedOwnerSnapshot())
}

// TestDisseminationSnapshotChangesWhenDestinationGrantEnds proves pointer
// reuse cannot keep an old publication after its complete grants disappear.
func TestDisseminationSnapshotChangesWhenDestinationGrantEnds(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	before := f.peer.selectedOwnerSnapshot()
	f.peer.mu.Lock()
	f.peer.settings.Bus = testBusNone()
	f.peer.mu.Unlock()
	f.peer.refreshForwardFacts()
	after := f.peer.selectedOwnerSnapshot()
	require.NotSame(t, before, after)
	require.False(t, after.owner.IsSet())
}

// TestDisseminationSettingsPreservesUnreadDynamicSources proves a BGP-section
// reload retains the source supplied by a dynamic group it did not read.
func TestDisseminationSettingsPreservesUnreadDynamicSources(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	r := f.api.r
	r.dynamicGroups = []*DynamicGroupConfig{{GroupName: "retained", Settings: &PeerSettings{
		Bus: testBusSourceLease(selectedWriterProcess),
	}}}
	destination := f.peer.settingsSnapshot()
	require.NoError(t, r.resolveDisseminationSettings([]*PeerSettings{destination}, dynamicGroupsNotRead))
	require.Equal(t, selectedWriterProcess, destination.DisseminationOwner.Name())

	// An explicitly read empty group set removes that source from the candidate.
	require.Error(t, r.resolveDisseminationSettings([]*PeerSettings{destination}, dynamicGroupsReadAs(nil)))
}

// TestDisseminationTemplatesRemainSeparate proves two groups with no endpoint
// keep independent grants in the configuration gate as well as at runtime.
func TestDisseminationTemplatesRemainSeparate(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	source := &PeerSettings{Bus: testBusSourceLease(selectedWriterProcess)}
	destination := f.peer.settingsSnapshot()
	destination.Address = netip.Addr{}
	nodes := DisseminationPeersFromSettings([]*PeerSettings{source, destination})
	require.NotEqual(t, nodes[0].Addr, nodes[1].Addr)
	owners, err := f.api.r.api.ResolveDisseminationOwners("bgp", SelectedDisseminationRole, nodes)
	require.NoError(t, err)
	require.Equal(t, selectedWriterProcess, owners[nodes[1].Addr].Name())
}

// TestDisseminationOwnerClassRestartsSameNamedProcess proves an ordinary owner
// becoming an explicit claimant changes session ownership despite an equal name.
func TestDisseminationOwnerClassRestartsSameNamedProcess(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	ordinary := f.peer.selectedOwnerSnapshot().owner
	graph := buildTestDisseminationGraph(t, []pluginserver.DeliveryPeer{
		testDeliveryPeer("source", "source", testBusSourceLease(selectedWriterProcess)),
		testDeliveryPeer("destination", "destination", testBusSelectedDest(selectedWriterProcess)),
	})
	declaration := registry.Lookup(selectedWriterProcess)
	require.NotNil(t, declaration)
	owner, err := graph.ResolveDisseminationOwner("destination", SelectedDisseminationRole, []pluginserver.DisseminationProcess{{
		Name: selectedWriterProcess, Claims: []string{SelectedDisseminationRole}, Declarations: declaration.Dissemination,
	}})
	require.NoError(t, err)
	require.Equal(t, ordinary.Name(), owner.Name())
	require.True(t, owner.Explicit())
	require.False(t, ordinary.Explicit())
	current := swapTestPeerSettings(t)
	current.Address = netip.MustParseAddr("192.0.2.80")
	current.DisseminationOwner = ordinary
	next := *current
	next.DisseminationOwner = owner
	require.Contains(t, peerSettingsRestartReason(current, &next, nil), "DisseminationOwner")
}
