package static

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	bfdapi "github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// stopTestBFDHandle is one BFD session whose subscriber channel the test
// drives, and closes the way engine.Loop.Stop does.
type stopTestBFDHandle struct {
	key bfdapi.Key
	ch  chan bfdapi.StateChange
}

// stopTestBFDService hands out one handle per EnsureSession.
type stopTestBFDService struct {
	handles []*stopTestBFDHandle
}

func (h *stopTestBFDHandle) Key() bfdapi.Key                              { return h.key }
func (h *stopTestBFDHandle) Subscribe() <-chan bfdapi.StateChange         { return h.ch }
func (h *stopTestBFDHandle) Unsubscribe(_ <-chan bfdapi.StateChange)      {}
func (h *stopTestBFDHandle) Shutdown() error                              { return nil }
func (h *stopTestBFDHandle) Enable() error                                { return nil }
func (s *stopTestBFDService) Snapshot() []bfdapi.SessionState             { return nil }
func (s *stopTestBFDService) Profiles() []bfdapi.ProfileState             { return nil }
func (s *stopTestBFDService) ReleaseSession(_ bfdapi.SessionHandle) error { return nil }
func (s *stopTestBFDService) SessionDetail(_ string) (bfdapi.SessionState, bool) {
	return bfdapi.SessionState{}, false
}

func (s *stopTestBFDService) EnsureSession(req bfdapi.SessionRequest) (bfdapi.SessionHandle, error) {
	h := &stopTestBFDHandle{key: req.Key(), ch: make(chan bfdapi.StateChange, 4)}
	s.handles = append(s.handles, h)
	return h, nil
}

// awaitNextHop polls the first next-hop of key until want accepts it.
func awaitNextHop(t *testing.T, rm *routeManager, key routeKey, what string, want func(nhState) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rm.mu.Lock()
		ok := want(rm.routes[key].nhStates[0])
		rm.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("next hop never reached %s", what)
}

// VALIDATES: when the BFD engine stops, which closes every subscriber channel,
// a BFD-tracked next hop forgets its handle and is active again, the state a
// next hop has when static runs without BFD.
//
// PREVENTS: a next hop that BFD had marked down staying out of the route for
// good once the engine is gone: nothing would ever report it Up again, and the
// next-hop kept the handle of a loop that no longer exists.
func TestStaticBFDEngineStopClearsTheNextHop(t *testing.T) {
	backend := &mockStaticBackend{}
	rm := newTestRouteManager(backend)
	svc := &stopTestBFDService{}
	rm.setBFD(svc)

	route := staticRoute{
		Prefix: netip.MustParsePrefix("10.9.0.0/16"),
		Action: actionForward,
		NextHops: []nextHop{
			{Address: netip.MustParseAddr("192.0.2.1"), Weight: 1, BFDProfile: "fast"},
			{Address: netip.MustParseAddr("192.0.2.2"), Weight: 1},
		},
	}
	if err := rm.applyRoutes([]staticRoute{route}); err != nil {
		t.Fatalf("applyRoutes: %v", err)
	}
	defer rm.shutdown()
	if len(svc.handles) != 1 {
		t.Fatalf("EnsureSession calls = %d, want 1", len(svc.handles))
	}
	key := routeKey{table: route.Table, prefix: route.Prefix}
	handle := svc.handles[0]

	handle.ch <- bfdapi.StateChange{Key: handle.key, State: bfdapi.StateDown, When: time.Now()}
	awaitNextHop(t, rm, key, "inactive after BFD Down", func(n nhState) bool { return !n.active })

	close(handle.ch)
	awaitNextHop(t, rm, key, "active with no handle after the engine stopped", func(n nhState) bool {
		return n.active && n.handle == nil && n.unsub == nil
	})

	// The route is programmed again with the next hop back in it: the last
	// Loc-RIB Path carries both next hops, one as its equal-cost sibling.
	rm.mu.Lock()
	last := backend.paths[len(backend.paths)-1]
	rm.mu.Unlock()
	if len(last.ECMP) != 1 {
		t.Fatalf("last Loc-RIB Path carries %d equal-cost siblings, want 1: the route was not reprogrammed", len(last.ECMP))
	}
}

// stopTestPluginServer is the daemon's plugin server as registry.DaemonStopping
// reads it: running, or with its Stop begun.
type stopTestPluginServer struct {
	registry.PluginServerAccessor
	running bool
}

func (s *stopTestPluginServer) Running() bool                       { return s.running }
func (s *stopTestPluginServer) SetMetricsRegistry(metrics.Registry) {}

// VALIDATES: a next hop whose BFD engine stopped warns that it is no longer
// BFD-tracked while the daemon runs, and does not once the plugin server has
// begun to stop; both forget the handle.
//
// PREVENTS: every daemon stop that happens to end the BFD plugin before static
// logging a warning that is false, because static stops a moment later. And a
// guard that silences the warning under a running daemon.
func TestStaticBFDEngineStopWarnsOnlyWhileDaemonRuns(t *testing.T) {
	for _, running := range []bool{true, false} {
		prev := registry.GetPluginServer()
		registry.SetPluginServer(&stopTestPluginServer{running: running})
		var sink bytes.Buffer
		prevLog := logger()
		setLogger(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn})))

		rm := newTestRouteManager(&mockStaticBackend{})
		svc := &stopTestBFDService{}
		rm.setBFD(svc)
		route := staticRoute{
			Prefix:   netip.MustParsePrefix("10.9.0.0/16"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("192.0.2.1"), Weight: 1, BFDProfile: "fast"}},
		}
		if err := rm.applyRoutes([]staticRoute{route}); err != nil {
			t.Fatalf("applyRoutes: %v", err)
		}
		key := routeKey{table: route.Table, prefix: route.Prefix}
		rm.endVanishedBFDSession(key, 0, svc.handles[0])

		rm.mu.Lock()
		cleared := rm.routes[key].nhStates[0].handle == nil
		rm.mu.Unlock()
		rm.shutdown()
		setLogger(prevLog)
		registry.SetPluginServer(prev)

		if !cleared {
			t.Fatalf("daemon running %v: the next hop kept its handle after the engine stopped", running)
		}
		warned := strings.Contains(sink.String(), "no longer BFD-tracked")
		if warned != running {
			t.Fatalf("daemon running %v: warned = %v, want %v; log: %q", running, warned, running, sink.String())
		}
	}
}
