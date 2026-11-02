// Design: docs/guide/graceful-restart.md -- GR retention precedes the forwarders' peer-down.
// Related: events_dependency_alias_test.go -- the delivery order by implementation identity

package server

import (
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/events"
	pluginrpc "github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRetentionEventPrecedesDown proves the D-11 guarantee end to end through
// the event dispatcher: bgp-gr, delivered a DOWN, emits its bgp-gr/retention
// verdict inside its handler, and bgp-rr receives that verdict BEFORE its own
// DOWN for the same peer, with bgp-gr internal and with bgp-gr forked.
//
// Method: three processes (bgp-gr, bgp-rr, bgp-rib) take delivery over a pipe.
// The gr responder emits the verdict through the server's bus before it
// acknowledges the DOWN; the rr responder records what it receives, in order.
// The order holds only if the DOWN reaches gr before rr (the reverse
// dependency tier, sortByReverseDependencyTier) and the verdict is queued on
// rr's one FIFO channel before the DOWN (process.Deliver). Forked, gr is an
// external program, so its edges are the ones its Stage 1 declaration carries
// (plugin.PluginRegistration), never a same-named registration's.
//
// RFC 4724 Section 4.2: "the Receiving Speaker MUST retain the routes received
// from the peer for all the address families that were previously received in
// the Graceful Restart Capability".
func TestRetentionEventPrecedesDown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		gr     plugin.PluginConfig
		stage1 *plugin.PluginRegistration
	}{
		{
			name: "bgp-gr internal",
			gr:   plugin.PluginConfig{Name: "bgp-gr", Internal: true, Run: "ze.bgp-gr"},
		},
		{
			name: "bgp-gr forked",
			gr:   plugin.PluginConfig{Name: "bgp-gr", Run: "ze plugin bgp-gr"},
			// What registrationFromRPC builds from gr's Stage 1 declaration.
			stage1: &plugin.PluginRegistration{
				Name:                 "bgp-gr",
				Dependencies:         []string{"bgp-rib"},
				OptionalDependencies: []string{"bgp-rs"},
				StartAfter:           []string{"bgp-rr"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newBarrierTestServer(t)

			grProc, grConn := newConfiguredProcWithConn(t, tc.gr)
			if tc.stage1 != nil {
				grProc.SetRegistration(tc.stage1)
			}
			rrProc, rrConn := newConfiguredProcWithConn(t, plugin.PluginConfig{Name: "bgp-rr", Internal: true, Run: "ze.bgp-rr"})
			ribProc, ribConn := newConfiguredProcWithConn(t, plugin.PluginConfig{Name: "bgp-rib", Internal: true, Run: "ze.bgp-rib"})
			for _, proc := range []*process.Process{grProc, rrProc, ribProc} {
				subscribeState(srv, proc)
			}
			srv.Subscriptions().Add(rrProc, &pluginserver.Subscription{
				Namespace: events.LookupNamespaceID(retention.Namespace),
				EventType: events.LookupEventTypeID(retention.EventType),
				Direction: events.DirUnspecified,
			})
			attachForDelivery(t, srv, testPeerInfo(), "bgp-gr", "bgp-rr", "bgp-rib")

			go retentionEmitter(t, grConn, srv)
			rr := &receivedLog{}
			go rr.record(t, rrConn)
			go rr.discard(t, ribConn)

			onPeerStateChange(srv, testPeerInfo(), pluginrpc.SessionStateDown, "tcp-failure")

			require.Eventually(t, func() bool { return rr.index(isDown) >= 0 }, 5*time.Second, 10*time.Millisecond,
				"bgp-rr never received the DOWN: %v", rr.all())
			verdict, down := rr.index(isVerdict), rr.index(isDown)
			require.GreaterOrEqual(t, verdict, 0, "bgp-rr never received the retention verdict: %v", rr.all())
			require.Less(t, verdict, down,
				"bgp-rr received its DOWN before bgp-gr's retention verdict, so it decides the DOWN without it: %v", rr.all())
		})
	}
}

// newConfiguredProcWithConn is newTestProcWithConn for a full plugin config:
// the delivery order reads Internal and Run, not only the name.
func newConfiguredProcWithConn(t *testing.T, cfg plugin.PluginConfig) (*process.Process, *plugipc.PluginConn) {
	t.Helper()

	engineSide, pluginSide := net.Pipe()
	proc := process.NewProcess(cfg)
	proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
	proc.StartDelivery(t.Context())
	t.Cleanup(func() {
		proc.Stop()
		if err := pluginSide.Close(); err != nil {
			t.Logf("close plugin side: %v", err) // Best-effort cleanup in test
		}
	})
	return proc, plugipc.NewPluginConn(pluginSide, pluginSide)
}

// retentionEmitter answers deliveries as bgp-gr does: on a DOWN it emits the
// retention verdict for the peer through the server's bus, then acknowledges.
func retentionEmitter(t *testing.T, conn *plugipc.PluginConn, srv *pluginserver.Server) {
	for {
		req, err := conn.ReadRequest(t.Context())
		if err != nil {
			return
		}
		if isDown(string(req.Params)) {
			verdict := &retention.Verdict{PeerAddress: testPeerInfo().AddrStr(), Families: []string{"ipv4/unicast"}}
			if _, err := srv.Emit(retention.Namespace, retention.EventType, verdict); err != nil {
				t.Errorf("emit retention verdict: %v", err)
			}
		}
		if err := conn.SendResult(t.Context(), req.ID, nil); err != nil {
			return
		}
	}
}

// isDown reports whether a delivery carries the peer's DOWN state event.
func isDown(params string) bool {
	return strings.Contains(params, "down") && !isVerdict(params)
}

// isVerdict reports whether a delivery carries the retention verdict.
func isVerdict(params string) bool { return strings.Contains(params, "peer-address") }

// receivedLog is the events one responder received, in arrival order.
// Safe for concurrent use.
type receivedLog struct {
	mu   sync.Mutex
	seen []string
}

func (l *receivedLog) record(t *testing.T, conn *plugipc.PluginConn) {
	for {
		req, err := conn.ReadRequest(t.Context())
		if err != nil {
			return
		}
		// One delivery carries a batch of events, in channel order.
		var batch struct {
			Events []string `json:"events"`
		}
		if err := json.Unmarshal(req.Params, &batch); err != nil {
			t.Errorf("delivery params %s: %v", req.Params, err)
			return
		}
		l.mu.Lock()
		l.seen = append(l.seen, batch.Events...)
		l.mu.Unlock()
		if err := conn.SendResult(t.Context(), req.ID, nil); err != nil {
			return
		}
	}
}

func (l *receivedLog) discard(t *testing.T, conn *plugipc.PluginConn) {
	sink := &receivedLog{}
	sink.record(t, conn)
}

func (l *receivedLog) index(match func(string) bool) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, params := range l.seen {
		if match(params) {
			return i
		}
	}
	return -1
}

func (l *receivedLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.seen...)
}
