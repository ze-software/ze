package rr

import (
	"encoding/json"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// sessionReadyCommand is the report bgp-rr owes a peer whose initial routing
// update it takes part in. The string is the engine's registered RPC surface
// (ze-plugin:session-peer-ready, bgp/plugins/cmd/peer/session.go), so the test
// asserts the wire text rather than the helper that builds it.
const sessionReadyCommand = "request peer 10.0.0.1 plugin session ready"

// sessionReadyHarness runs one routeReflector over a DirectBridge and records
// every command it dispatches, on both rails: the replay uses
// DispatchCommandArgs and the session-ready report uses DispatchCommand, so a
// test that hooks only one of them cannot tell a missing report from a
// captured one.
type sessionReadyHarness struct {
	rr *routeReflector

	mu       sync.Mutex
	commands []string

	// replayErr is returned from the replay rail, which drives replayForPeer
	// down its early-return path.
	replayErr error
}

// newSessionReadyHarness builds the harness with one peer at generation 1 and
// no negotiated family, so sendEOR returns before it writes anything and the
// only command left to observe is the report itself.
func newSessionReadyHarness(t *testing.T) *sessionReadyHarness {
	t.Helper()

	harness := &sessionReadyHarness{}

	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })

	plugin := sdk.NewWithConn("rr-session-ready-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = plugin.Close() })

	bridge.SetDispatchCommandArgs(func(command string, _ []string, _ string) (*rpc.DispatchCommandOutput, error) {
		harness.record(command)
		if harness.replayErr != nil {
			return nil, harness.replayErr
		}
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"last-index":0,"replayed":0}`)}, nil
	})
	bridge.SetDispatchCommand(func(command string) (*rpc.DispatchCommandOutput, error) {
		harness.record(command)
		return &rpc.DispatchCommandOutput{Status: statusDone}, nil
	})
	bridge.SetReady()

	harness.rr = &routeReflector{
		plugin: plugin,
		peers: map[string]*peerState{
			"10.0.0.1": {
				Address:   "10.0.0.1",
				Up:        true,
				ReplayGen: 1,
				Families:  map[family.Family]bool{},
			},
		},
		withdrawals: make(map[string]map[string]withdrawalInfo),
	}
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

// TestRRReportsSessionReadyOnEveryPathOutOfReplay pins the report bgp-rr owes
// the peer's initial-sync barrier.
//
// RFC 4724 Section 4: "The End-of-RIB marker MUST be sent by a BGP speaker to
// its peer once it completes the initial routing update (including the case
// when there is no update to send) for an address family after the BGP session
// is established". bgp-rr declares Registration.SignalsSessionReady
// (rr/register.go), so a peer that attaches it with a route-push grant and a
// peer-state grant NAMES it in that peer's barrier
// (Peer.initialUpdateReporters, bgp/reactor/peer_run.go) and holds the marker
// until this report arrives.
//
// VALIDATES: replayForPeer reports on the path where the replay succeeds AND on
// the path where the replay fails and it returns early.
// PREVENTS: the declaration outliving the report. bgp-rr declared and did not
// report at all until 2026-09-02, and every peer that attached it then waited
// out apiSyncTimeout before its End-of-RIB. The early return is the path that
// makes this more than a formality: it is reached whenever the adj-rib-in
// replay rail answers an error, and a report placed after the replay rather
// than deferred would be skipped exactly there.
func TestRRReportsSessionReadyOnEveryPathOutOfReplay(t *testing.T) {
	t.Run("the replay succeeds", func(t *testing.T) {
		harness := newSessionReadyHarness(t)

		harness.rr.replayForPeer("10.0.0.1", 1)

		assert.True(t, harness.reported(),
			"a completed replay must report, or this peer's End-of-RIB waits out the api sync timeout")
	})

	t.Run("the replay rail answers an error", func(t *testing.T) {
		harness := newSessionReadyHarness(t)
		harness.replayErr = errors.New("adj-rib-in replay refused")

		harness.rr.replayForPeer("10.0.0.1", 1)

		assert.True(t, harness.reported(),
			"a failed replay still ends what this plugin owes the initial routing update, so it must report")
	})
}

// TestRRDoesNotReportForAStaleGeneration pins the other direction of the same
// report.
//
// VALIDATES: a replay whose peer has re-established since it started reports
// nothing.
// PREVENTS: releasing the CURRENT session's End-of-RIB on work done for the
// previous one. ReplayGen is bumped on every state-up (handleState, rr.go), so
// a rapid reconnect leaves the old goroutine running against a barrier that
// belongs to a session it never replayed for.
func TestRRDoesNotReportForAStaleGeneration(t *testing.T) {
	harness := newSessionReadyHarness(t)

	// The peer sits at generation 1; this goroutine belongs to generation 0.
	harness.rr.replayForPeer("10.0.0.1", 0)

	require.False(t, harness.reported(),
		"a stale replay must not report: the barrier it would release belongs to a session it did not serve")
}
