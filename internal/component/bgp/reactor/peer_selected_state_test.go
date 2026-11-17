// Design: docs/architecture/api/process-protocol.md -- selected destination identity.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestSelectedDestinationUsesWholeSessionToken proves output identity survives
// initial EOR. Method: use an actual eligible writer fixture, close its initial
// replay phase, then project a later attached-UP's stable connection metadata.
func TestSelectedDestinationUsesWholeSessionToken(t *testing.T) {
	fixture := newSelectedWriterFixture(t, false)
	fixture.peer.session.initialSyncEOROwed.Store(false)
	fixture.peer.initialUpdateOwed.Store(false)
	info := establishedPeerInfo(fixture.peer)
	require.Zero(t, info.InitialReplay)
	require.Equal(t, uint64(71), info.DestinationSession)
	require.Equal(t, selectedWriterProcess, info.SelectedOwner)
}

// TestSelectedDestinationYieldsToExplicitRole proves a complete claimant does
// not also start ordinary output, even with the same configured process name.
// Method: resolve the explicit class through the real graph and publish it on
// the writer fixture's existing exact-session snapshot.
func TestSelectedDestinationYieldsToExplicitRole(t *testing.T) {
	fixture := newSelectedWriterFixture(t, false)
	declaration := registry.Lookup(selectedWriterProcess).Dissemination
	graph := fixture.peer.reactor.api.DeliveryGraph()
	owner, err := graph.ResolveDisseminationOwner("192.0.2.80", SelectedDisseminationRole,
		[]pluginserver.DisseminationProcess{{Name: selectedWriterProcess,
			Claims: []string{SelectedDisseminationRole}, Declarations: declaration}})
	require.NoError(t, err)
	require.True(t, owner.Explicit())
	facts := *fixture.peer.forwardFacts()
	snapshot := *facts.dissemination
	snapshot.owner = owner
	facts.dissemination = &snapshot
	fixture.peer.fwdFacts.Store(&facts)
	info := establishedPeerInfo(fixture.peer)
	require.Equal(t, uint64(71), info.DestinationSession)
	require.Empty(t, info.SelectedOwner)
}
