package persist

import (
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// sessionReadyCommand is the report bgp-persist owes a peer whose initial
// routing update it takes part in. The string is the engine's registered RPC
// surface (ze-plugin:session-peer-ready, bgp/plugins/cmd/peer/session.go), so
// the test asserts the wire text rather than the helper that builds it.
const sessionReadyCommand = "request peer 10.0.0.1 plugin session ready"

// sessionReadyHarness runs one PersistServer over a DirectBridge and records
// every command it dispatches.
//
// The two rails are kept apart on purpose. Routes and End-of-RIB go through
// updateRouteHook, which is where the existing tests read them, and the
// session-ready report goes through the bridge, which is the only rail
// signalSessionReady uses. Leaving ps.plugin nil, as newTestPersistServer
// does, makes that function return at its first line, so a test built on the
// hook alone cannot tell a missing report from a captured one.
type sessionReadyHarness struct {
	ps *PersistServer

	mu       sync.Mutex
	commands []string
}

func newSessionReadyHarness(t *testing.T) *sessionReadyHarness {
	t.Helper()

	harness := &sessionReadyHarness{}

	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })

	plugin := sdk.NewWithConn("persist-session-ready-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = plugin.Close() })

	bridge.SetDispatchCommand(func(command string) (*rpc.DispatchCommandOutput, error) {
		harness.record(command)
		return &rpc.DispatchCommandOutput{Status: "done"}, nil
	})
	bridge.SetReady()

	harness.ps = &PersistServer{
		peers:  make(map[string]*PersistPeer),
		ribOut: make(map[string]map[family.Family]map[string]*StoredRoute),
		plugin: plugin,
	}
	harness.ps.updateRouteHook = func(peer, cmd string) { harness.record(peer + "\t" + cmd) }
	return harness
}

func (h *sessionReadyHarness) record(command string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.commands = append(h.commands, command)
}

func (h *sessionReadyHarness) reported() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Contains(h.commands, sessionReadyCommand)
}

// addPeer puts one established peer in the server at generation 0, with the
// families the replay will send an End-of-RIB for.
func (h *sessionReadyHarness) addPeer(routes map[string]*StoredRoute) {
	h.ps.mu.Lock()
	defer h.ps.mu.Unlock()
	h.ps.peers["10.0.0.1"] = &PersistPeer{
		Address:  "10.0.0.1",
		Up:       true,
		Families: map[family.Family]bool{family.IPv4Unicast: true},
	}
	if routes != nil {
		h.ps.ribOut["10.0.0.1"] = map[family.Family]map[string]*StoredRoute{family.IPv4Unicast: routes}
	}
}

// TestPersistReportsSessionReadyOnBothCompletionPaths pins the report
// bgp-persist owes the peer's initial-sync barrier.
//
// RFC 4724 Section 4: "The End-of-RIB marker MUST be sent by a BGP speaker to
// its peer once it completes the initial routing update (including the case
// when there is no update to send) for an address family after the BGP session
// is established". bgp-persist declares Registration.SignalsSessionReady
// (persist/register.go), so a peer that attaches it with a route-push grant and
// a peer-state grant NAMES it in that peer's barrier
// (Peer.initialUpdateReporters, bgp/reactor/peer_run.go) and holds the marker
// until this report arrives.
//
// VALIDATES: replayForPeer reports on the path that replays stored routes AND
// on the path that has nothing stored and sends the End-of-RIB alone.
// PREVENTS: the declaration outliving the report. bgp-persist declared and did
// not report at all until 2026-09-02, and every peer that attached it then
// waited out apiSyncTimeout before its End-of-RIB. The empty path is the one
// that makes this more than a formality: a barrier cannot tell "finished with
// nothing to send" from "still working", so the peer with nothing stored is
// exactly the peer that stalls when the report is placed after the routes.
func TestPersistReportsSessionReadyOnBothCompletionPaths(t *testing.T) {
	t.Run("stored routes are replayed", func(t *testing.T) {
		harness := newSessionReadyHarness(t)
		harness.addPeer(map[string]*StoredRoute{
			"prefix 192.168.1.0/24": {MsgID: 42, Family: family.IPv4Unicast, Prefix: "prefix 192.168.1.0/24"},
		})

		harness.ps.replayForPeer("10.0.0.1", 0)

		assert.True(t, harness.reported(),
			"a completed replay must report, or this peer's End-of-RIB waits out the api sync timeout")
	})

	t.Run("nothing is stored for the peer", func(t *testing.T) {
		harness := newSessionReadyHarness(t)
		harness.addPeer(nil)

		harness.ps.replayForPeer("10.0.0.1", 0)

		assert.True(t, harness.reported(),
			"an empty replay still completes this plugin's share of the initial routing update, so it must report")
	})
}

// TestPersistDoesNotReportForAStaleGeneration pins the other direction of the
// same report.
//
// VALIDATES: a replay whose peer has re-established since it started reports
// nothing.
// PREVENTS: releasing the CURRENT session's End-of-RIB on work done for the
// previous one. replayGen is bumped on every state-up (handleState and
// handleStructuredState, server.go), and the replay that owns the new session
// sends the report for it.
func TestPersistDoesNotReportForAStaleGeneration(t *testing.T) {
	harness := newSessionReadyHarness(t)
	harness.addPeer(nil)

	// The peer sits at generation 0; this goroutine belongs to generation 1.
	harness.ps.replayForPeer("10.0.0.1", 1)

	require.False(t, harness.reported(),
		"a stale replay must not report: the barrier it would release belongs to a session it did not serve")
}

// TestPersistReportsSessionReadyFromThePeerUpEvent walks the whole path the
// engine drives, from the peer-up event a `receive [ state ]` grant delivers to
// the report that releases the barrier.
//
// VALIDATES: the state-up text event reaches the report, not just replayForPeer
// called by hand.
// PREVENTS: a report reachable only from a helper. The barrier is armed by the
// reactor at Established and answered by whatever the peer-up event triggers,
// so a report the event never reaches is a report that never arrives.
func TestPersistReportsSessionReadyFromThePeerUpEvent(t *testing.T) {
	harness := newSessionReadyHarness(t)
	harness.addPeer(nil)

	harness.ps.dispatchText("peer 10.0.0.1 remote as 65001 state up")

	require.Eventually(t, harness.reported, 2*time.Second, time.Millisecond,
		"the peer-up event must reach the session-ready report through the replay goroutine")
}
