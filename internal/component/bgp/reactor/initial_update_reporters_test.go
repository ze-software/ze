package reactor

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
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

// sendUpdateBinding is a binding that grants the route-push rail, with the peer
// state grant chosen by the caller.
func sendUpdateBinding(name string, receivesState bool) ProcessBinding {
	b := ProcessBinding{
		PluginName: name,
		Send:       map[string]bool{bgpevents.SendUpdate: true},
	}
	if receivesState {
		b.Receive = map[string]events.Direction{bgpevents.EventState: events.DirBoth}
	}
	return b
}

// peerWithBindings builds the minimum Peer the barrier population reads.
func peerWithBindings(bindings ...ProcessBinding) *Peer {
	return &Peer{settings: &PeerSettings{
		Address:         netip.MustParseAddr("192.0.2.1"),
		ProcessBindings: bindings,
	}}
}

// VALIDATES: a peer's initial-sync barrier names exactly the processes that
// hold a route-push grant, declare Registration.SignalsSessionReady, AND are
// told the session came up.
// PREVENTS: the two opposite failures this barrier can produce. Naming a
// process that will never report holds the peer's End-of-RIB to the api-sync
// timeout on every establishment, which is the stall a plugin bound
// `send [ update ]` with no `receive [ state ]` used to cause. Naming none of
// them sends the marker while a declaring plugin is still writing the routes
// the marker claims are complete (RFC 4724 Section 4).
func TestInitialUpdateReporters(t *testing.T) {
	declaring, silent := registerReporterTestPlugins(t)

	tests := []struct {
		name     string
		bindings []ProcessBinding
		want     []string
	}{
		{
			name:     "declaring plugin with both grants is waited for",
			bindings: []ProcessBinding{sendUpdateBinding(declaring, true)},
			want:     []string{declaring},
		},
		{
			name:     "declaring plugin with no peer-state grant is not waited for",
			bindings: []ProcessBinding{sendUpdateBinding(declaring, false)},
			want:     nil,
		},
		{
			name:     "non-declaring plugin is not waited for",
			bindings: []ProcessBinding{sendUpdateBinding(silent, true)},
			want:     nil,
		},
		{
			name:     "unregistered plugin declares nothing and is not waited for",
			bindings: []ProcessBinding{sendUpdateBinding("test-never-registered-reporter", true)},
			want:     nil,
		},
		{
			name: "no route-push grant is not waited for",
			bindings: []ProcessBinding{{
				PluginName: declaring,
				Receive:    map[string]events.Direction{bgpevents.EventState: events.DirBoth},
			}},
			want: nil,
		},
		{
			name: "the wildcard receive list carries the peer-state grant",
			bindings: []ProcessBinding{{
				PluginName: declaring,
				Send:       map[string]bool{bgpevents.SendUpdate: true},
				ReceiveAll: true,
			}},
			want: []string{declaring},
		},
		{
			name: "the raw rail is a route-push grant too",
			bindings: []ProcessBinding{{
				PluginName: declaring,
				Send:       map[string]bool{bgpevents.SendRaw: true},
				Receive:    map[string]events.Direction{bgpevents.EventState: events.DirBoth},
			}},
			want: []string{declaring},
		},
		{
			name: "one qualifying binding beside one that does not",
			bindings: []ProcessBinding{
				sendUpdateBinding(silent, true),
				sendUpdateBinding(declaring, true),
			},
			want: []string{declaring},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peerWithBindings(tt.bindings...).initialUpdateReporters()
			require.Equal(t, tt.want, got)
		})
	}
}

// VALIDATES: a process the compile-time registry does not hold is answered for
// by the runtime seam, which is the only route an EXTERNAL plugin has to the
// session-ready declaration.
// PREVENTS: an external plugin that DOES report being unreachable by the
// barrier. Before the seam existed the reactor answered false for every
// unregistered name, so a plugin injecting a route through `send [ raw ]` could
// not be waited for and its route raced the marker that claims the initial
// routing update is complete (test/plugin/initial-sync-barrier-raw.ci).
func TestInitialUpdateReportersReadsTheRuntimeDeclaration(t *testing.T) {
	const external = "test-external-session-ready-process"

	peer := peerWithBindings(sendUpdateBinding(external, true))
	require.Empty(t, peer.initialUpdateReporters(), "no seam: an unregistered process declares nothing")

	registry.SetRuntimeSessionReady(func(process string) bool { return process == external })
	t.Cleanup(func() { registry.SetRuntimeSessionReady(nil) })

	require.Equal(t, []string{external}, peer.initialUpdateReporters())

	// The seam answers per process, not for every unregistered name.
	other := peerWithBindings(sendUpdateBinding("test-external-silent-process", true))
	require.Empty(t, other.initialUpdateReporters())
}

// VALIDATES: the name the declaration is looked up under is the name the
// binding carries, which is the operator's `attach process <name>` key and the
// same name the barrier holds and credits reports to.
// PREVENTS: the two namespaces being read as one here. A process name is a
// registry key only when the operator did not rename the implementation:
// `internal rs { use bgp-rs }` runs the process as "rs" and files the
// registration under "bgp-rs". This package must keep asking under the process
// name, because that is what a report carries; resolving it to the
// implementation is the plugin server's job, and its own red is
// TestDeclaresSessionReadyResolvesTheProcessAlias
// (internal/component/plugin/server/events_session_ready_test.go).
func TestInitialUpdateReportersAsksUnderTheProcessName(t *testing.T) {
	const alias = "test-aliased-session-ready-process"

	var asked []string
	registry.SetRuntimeSessionReady(func(process string) bool {
		asked = append(asked, process)
		return process == alias
	})
	t.Cleanup(func() { registry.SetRuntimeSessionReady(nil) })

	peer := peerWithBindings(sendUpdateBinding(alias, true))
	require.Equal(t, []string{alias}, peer.initialUpdateReporters())
	require.Equal(t, []string{alias}, asked,
		"the declaration must be asked for under the name the barrier will hold")
}

// VALIDATES: ReceivesPeerState answers for the state event alone.
// PREVENTS: reading any receive grant as the peer-up grant, which would put a
// process that only takes UPDATEs into the barrier and stall the peer.
func TestReceivesPeerState(t *testing.T) {
	updateOnly := ProcessBinding{Receive: map[string]events.Direction{bgpevents.EventUpdate: events.DirReceived}}
	require.False(t, updateOnly.ReceivesPeerState())

	stateReceived := ProcessBinding{Receive: map[string]events.Direction{bgpevents.EventState: events.DirReceived}}
	require.True(t, stateReceived.ReceivesPeerState())

	none := ProcessBinding{}
	require.False(t, none.ReceivesPeerState())
}
