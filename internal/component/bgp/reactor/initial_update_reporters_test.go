package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

// registerReporterTestPlugins puts one declaring and one non-declaring plugin
// in the registry. Names are test-local so they cannot collide with a real
// plugin; the registry has no unregister, which is why they must stay
// distinctive.
func registerReporterTestPlugins(t *testing.T) (declaring, silent string) {
	t.Helper()

	declaring = "test-session-ready-declaring-plugin"
	silent = "test-session-ready-silent-plugin"

	base := func(name string, declares bool) registry.Registration {
		return registry.Registration{
			Name:                name,
			Description:         "session-ready declaration test fixture",
			SignalsSessionReady: declares,
			RunEngine:           func(_ net.Conn) int { return 0 },
			CLIHandler:          func(_ []string) int { return 0 },
		}
	}
	if err := registry.Register(base(declaring, true)); err != nil {
		require.ErrorIs(t, err, registry.ErrDuplicateName, "unexpected registration failure")
	}
	if err := registry.Register(base(silent, false)); err != nil {
		require.ErrorIs(t, err, registry.ErrDuplicateName, "unexpected registration failure")
	}
	return declaring, silent
}

// sendUpdateGrant is a grant for the route-push rail, with the peer state
// grant chosen by the caller.
func sendUpdateGrant(name string, receivesState bool) testPeerEndpointGrant {
	g := testPeerEndpointGrant{
		Name: name,
		Send: []string{bgpevents.SendUpdate},
	}
	if receivesState {
		g.Receive = []string{bgpevents.EventState}
	}
	return g
}

// peerWithBusGrants builds the minimum Peer with a compiled DeliveryGraph that
// initialUpdateReporters reads.
func peerWithBusGrants(t *testing.T, grants ...testPeerEndpointGrant) *Peer {
	t.Helper()
	peer := NewPeer(&PeerSettings{
		Address: netip.MustParseAddr("192.0.2.1"),
	})
	setTestPeerBus(t, peer, testBusBlockForGrants(grants...))
	return peer
}

// VALIDATES: a peer's session-ready expectation set names exactly the
// processes that hold a route-push grant, declare
// Registration.SignalsSessionReady, AND are told the session came up.
// PREVENTS: the two opposite failures this set can produce. Naming a process
// that will never report keeps any replay fence it owns up for the whole
// session. Naming none of them never raises the fence, so a live forward can
// overtake a declaring plugin's peer-up replay.
func TestInitialUpdateReporters(t *testing.T) {
	declaring, silent := registerReporterTestPlugins(t)

	tests := []struct {
		name   string
		grants []testPeerEndpointGrant
		want   []string
	}{
		{
			name:   "declaring plugin with both grants is waited for",
			grants: []testPeerEndpointGrant{sendUpdateGrant(declaring, true)},
			want:   []string{declaring},
		},
		{
			name:   "declaring plugin with no peer-state grant is not waited for",
			grants: []testPeerEndpointGrant{sendUpdateGrant(declaring, false)},
			want:   nil,
		},
		{
			name:   "non-declaring plugin is not waited for",
			grants: []testPeerEndpointGrant{sendUpdateGrant(silent, true)},
			want:   nil,
		},
		{
			name:   "unregistered plugin declares nothing and is not waited for",
			grants: []testPeerEndpointGrant{sendUpdateGrant("test-never-registered-reporter", true)},
			want:   nil,
		},
		{
			name: "no route-push grant is not waited for",
			grants: []testPeerEndpointGrant{{
				Name:    declaring,
				Receive: []string{bgpevents.EventState},
			}},
			want: nil,
		},
		{
			name: "the wildcard receive list carries the peer-state grant",
			grants: []testPeerEndpointGrant{{
				Name:       declaring,
				Send:       []string{bgpevents.SendUpdate},
				ReceiveAll: true,
			}},
			want: []string{declaring},
		},
		{
			name: "the raw rail is a route-push grant too",
			grants: []testPeerEndpointGrant{{
				Name:    declaring,
				Send:    []string{bgpevents.SendRaw},
				Receive: []string{bgpevents.EventState},
			}},
			want: []string{declaring},
		},
		{
			name: "one qualifying binding beside one that does not",
			grants: []testPeerEndpointGrant{
				sendUpdateGrant(silent, true),
				sendUpdateGrant(declaring, true),
			},
			want: []string{declaring},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peerWithBusGrants(t, tt.grants...).initialUpdateReporters()
			require.Equal(t, tt.want, got)
		})
	}
}

// VALIDATES: a process the compile-time registry does not hold is answered for
// by the runtime seam, which is the only route an EXTERNAL plugin has to the
// session-ready declaration.
// PREVENTS: an external plugin that DOES report being unreachable by the
// barrier.
func TestInitialUpdateReportersReadsTheRuntimeDeclaration(t *testing.T) {
	const external = "test-external-session-ready-process"

	peer := peerWithBusGrants(t, sendUpdateGrant(external, true))
	require.Empty(t, peer.initialUpdateReporters(), "no seam: an unregistered process declares nothing")

	registry.SetRuntimeSessionReady(func(process string) bool { return process == external })
	t.Cleanup(func() { registry.SetRuntimeSessionReady(nil) })

	require.Equal(t, []string{external}, peer.initialUpdateReporters())

	// The seam answers per process, not for every unregistered name.
	other := peerWithBusGrants(t, sendUpdateGrant("test-external-silent-process", true))
	require.Empty(t, other.initialUpdateReporters())
}

// VALIDATES: the name the declaration is looked up under is the endpoint name
// carried by the compiled DeliveryGraph and credited by the barrier.
func TestInitialUpdateReportersAsksUnderTheProcessName(t *testing.T) {
	const alias = "test-aliased-session-ready-process"

	var asked []string
	registry.SetRuntimeSessionReady(func(process string) bool {
		asked = append(asked, process)
		return process == alias
	})
	t.Cleanup(func() { registry.SetRuntimeSessionReady(nil) })

	peer := peerWithBusGrants(t, sendUpdateGrant(alias, true))
	require.Equal(t, []string{alias}, peer.initialUpdateReporters())
	require.Equal(t, []string{alias}, asked,
		"the declaration must be asked for under the name the barrier will hold")
}

// VALIDATES: ProcessReceivesState on DeliveryGraph answers for the state event
// alone.
// PREVENTS: reading any receive grant as the peer-up grant, which would put a
// process that only takes UPDATEs into the barrier and stall the peer.
func TestReceivesPeerState(t *testing.T) {
	updateOnly := peerWithBusGrants(t, testPeerEndpointGrant{Name: "p", Receive: []string{"update-received"}})
	require.False(t, updateOnly.reactor.DeliveryGraph().ProcessReceivesState(updateOnly.addrString, "p"))

	stateReceived := peerWithBusGrants(t, testPeerEndpointGrant{Name: "p", Receive: []string{bgpevents.EventState}})
	require.True(t, stateReceived.reactor.DeliveryGraph().ProcessReceivesState(stateReceived.addrString, "p"))

	none := peerWithBusGrants(t)
	require.False(t, none.reactor.DeliveryGraph().ProcessReceivesState(none.addrString, "p"))
}
