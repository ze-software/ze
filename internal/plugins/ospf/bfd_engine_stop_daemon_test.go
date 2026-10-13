package ospf

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// daemonStopTestPluginServer is the daemon's plugin server as
// registry.DaemonStopping reads it: running, or with its Stop begun.
type daemonStopTestPluginServer struct {
	registry.PluginServerAccessor
	running bool
}

func (s *daemonStopTestPluginServer) Running() bool                       { return s.running }
func (s *daemonStopTestPluginServer) SetMetricsRegistry(metrics.Registry) {}

// VALIDATES: an OSPF BFD client whose engine stopped warns that the neighbor
// runs on hello/dead timers while the daemon runs, and does not once the plugin
// server has begun to stop; both detach the client.
//
// PREVENTS: every daemon stop that happens to end the BFD plugin before OSPF
// logging a warning that is false, because OSPF stops a moment later. And a
// guard that silences the warning under a running daemon.
//
// The warning is logged before the client detaches under bfdMu, so reading the
// sink after the detach is observed under bfdMu reads every line it wrote.
func TestOSPFBFDEngineStopWarnsOnlyWhileDaemonRuns(t *testing.T) {
	for _, running := range []bool{true, false} {
		prev := registry.GetPluginServer()
		registry.SetPluginServer(&daemonStopTestPluginServer{running: running})

		svc := newFakeBFDService()
		e, _ := bfdTestEngine(t, enabledBFD(), svc)
		var sink bytes.Buffer
		e.log = slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
		id := ridMust(t, bfdTestPeerRID)
		seedNeighbor(t, e, id, netip.MustParseAddr(bfdTestPeerAddr))
		e.bfdNeighborFull(fullSnap(id))
		key := svc.firstRequest(t).Key()
		h := svc.handleFor(t, key)

		// engine.Loop.Stop: every subscriber channel closes.
		svc.mu.Lock()
		delete(svc.handles, key)
		svc.mu.Unlock()
		close(h.ch)

		left := -1
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			e.bfdMu.Lock()
			left = len(e.bfdClients)
			e.bfdMu.Unlock()
			if left == 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		registry.SetPluginServer(prev)

		if left != 0 {
			t.Fatalf("daemon running %v: BFD clients = %d 2s after the engine stopped, want 0", running, left)
		}
		warned := strings.Contains(sink.String(), "runs on hello/dead timers")
		if warned != running {
			t.Fatalf("daemon running %v: warned = %v, want %v; log: %q", running, warned, running, sink.String())
		}
	}
}
