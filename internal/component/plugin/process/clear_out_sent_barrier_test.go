// Design: docs/architecture/bgp/replay-cursor.md -- refresh reads the applied sent projection.
// Related: recovery_delivery_export_test.go -- FIFO gate without replacing handlers.
// Related: recovery_delivery_integration_test.go -- real RIB, TCP and transport fixtures.

package process_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The independent SENT monitor models what the bridge can already observe
// while the real RIB's separate delivery queue is held. It does not run the
// Python helper; api-rr-rib is the separate complete bridge/wire proof.
// This bootstrap negotiates no Enhanced Refresh capability, so this test asks
// only whether clear-out replays both prior admissions, not about markers.
func TestRIBClearOutWaitsForQueuedSent(t *testing.T) {
	for _, mode := range []string{"direct", "ipc"} {
		for _, held := range []bool{false, true} {
			name := "applied-control"
			if held {
				name = "held-sent"
			}
			t.Run(mode+"/"+name, func(t *testing.T) { clearOutSentScenario(t, mode, held) })
		}
	}
}

func clearOutSentScenario(t *testing.T, mode string, held bool) {
	t.Helper()
	clearOutSentScenarioOutcome(t, mode, held, false)
}

// Cancellation while the applied-event receipt is held must refuse the command
// without dispatching it. The same real owner is then released and joined.
func TestRIBClearOutCanceledQueuedSent(t *testing.T) {
	for _, mode := range []string{"direct", "ipc"} {
		t.Run(mode, func(t *testing.T) { clearOutSentScenarioOutcome(t, mode, true, true) })
	}
}

func clearOutSentScenarioOutcome(t *testing.T, mode string, held, cancelHeld bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	executed := make(chan struct{})
	if mode == "ipc" {
		restore, err := registry.WrapRunnerForTest("bgp-rib", func(conn net.Conn) net.Conn {
			return &clearOutSentConn{Conn: conn, executed: executed}
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := restore(); err != nil {
				t.Error(err)
			}
		})
	}
	r, srv, peers := recoveryDeliveryRouter(t, ctx)
	proc := srv.ProcessManager().GetProcess("bgp-rib")
	if proc == nil {
		t.Fatal("fixture did not start the registered RIB")
	}
	if proc.HasStructuredHandler() != (mode == "direct") {
		t.Fatalf("RIB transport does not match %s", mode)
	}
	monitor := pluginserver.NewMonitorClient(ctx, "clear-out-sent", []*pluginserver.Subscription{{
		Namespace: events.LookupNamespaceID("bgp"), EventType: events.LookupEventTypeID("update"),
		Direction: events.DirSent, PeerFilter: &pluginserver.PeerFilter{Selector: "127.0.0.1"},
	}}, 32)
	srv.Monitors().Add(monitor)
	t.Cleanup(func() { srv.Monitors().Remove("clear-out-sent") })
	api, ok := r.ReactorLifecycleAdapter().(interface {
		AnnounceNLRIBatch(context.Context, *selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error
	})
	if !ok {
		t.Fatal("real reactor has no batch announcement API")
	}
	announce := func(last byte) {
		t.Helper()
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{192, 168, 0, last}), 32)
		batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast,
			NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, prefix, 0, false)},
			NextHop: bgptypes.NewNextHopExplicit(netip.AddrFrom4([4]byte{10, 0, 0, last}))}
		if err := api.AnnounceNLRIBatch(ctx, selector.ParseDefault("127.0.0.1"), batch, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		if got := clearOutSentWire(t, ctx, peers[0]); got != last {
			t.Fatalf("initial wire route = %d, want %d", got, last)
		}
		clearOutSentObserved(t, ctx, monitor, last)
	}
	announce(2)
	if err := proc.DrainEventsApplied(ctx); err != nil {
		t.Fatal(err)
	}
	gate := process.GateRecoveryDelivery(t, ctx, proc, func(item process.EventDelivery) bool {
		return recoverySentFrom(item, "127.0.0.1", "")
	})
	var completed chan error
	joined := false
	t.Cleanup(func() {
		gate.Release()
		cancel()
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if completed != nil && !joined {
			select {
			case <-completed:
			case <-join.Done():
				t.Error("clear-out command did not join")
			}
		}
		r.Stop()
		srv.Stop()
		if err := gate.Wait(join); err != nil {
			t.Error(err)
		}
	})
	if !held {
		gate.Release()
	}
	announce(3)
	recoveryAwait(t, ctx, gate.Held, "last SENT admitted before the RIB applies it")
	if !held {
		if err := proc.DrainEventsApplied(ctx); err != nil {
			t.Fatal(err)
		}
		recoveryAwait(t, ctx, gate.Barriers, "control applied receipt")
	}
	completed = make(chan error, 1)
	commandCtx, cancelCommand := context.WithCancel(ctx)
	defer cancelCommand()
	go func() {
		response, err := srv.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: srv,
			RequestContext: commandCtx, Sender: plugin.OperatorSender()}, "clear bgp rib out 127.0.0.1")
		if err == nil {
			if response == nil {
				err = fmt.Errorf("clear-out returned no response")
			} else if response.Status != plugin.StatusDone {
				err = fmt.Errorf("clear-out returned status %s: %s", response.Status, response.Error)
			}
		}
		completed <- err
	}()
	if held {
		select {
		case <-gate.Barriers:
		case <-executed:
			t.Fatal("clear-out reached the RIB before its queued SENT event was applied")
		case err := <-completed:
			joined = true
			t.Fatalf("clear-out completed before its queued SENT event was applied: %v", err)
		case <-ctx.Done():
			t.Fatal("clear-out reached neither an applied-event barrier nor its handler", ctx.Err())
		}
		if cancelHeld {
			cancelCommand()
			err := recoveryResult(t, ctx, completed)
			joined = true
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled clear-out returned %v, want its cancellation", err)
			}
			select {
			case <-executed:
				t.Fatal("canceled clear-out reached the IPC handler before application")
			default:
			}
			return
		}
		gate.Release()
	}
	if err := recoveryResult(t, ctx, completed); err != nil {
		joined = true
		t.Fatal(err)
	}
	joined = true
	seen := make(map[byte]bool, 2)
	for range 2 {
		last := clearOutSentWire(t, ctx, peers[0])
		if seen[last] {
			t.Fatalf("replay duplicated route %d instead of replaying both prior admissions", last)
		}
		seen[last] = true
	}
	// Replay creates real SENT feedback. It must remain asynchronous and drain
	// after the command completes, rather than deadlocking its own callback.
	if err := proc.DrainEventsApplied(ctx); err != nil {
		t.Fatal(err)
	}
}

func clearOutSentWire(t *testing.T, ctx context.Context, peer *recoveryTCPPeer) byte {
	t.Helper()
	update := recoveryNextUpdate(t, ctx, peer)
	if len(update.NLRI) != 5 {
		t.Fatalf("wire NLRI = %x, want one scripted host route", update.NLRI)
	}
	if !bytes.Equal(update.NLRI[:4], []byte{32, 192, 168, 0}) {
		t.Fatalf("wire NLRI = %x, want a scripted host route", update.NLRI)
	}
	last := update.NLRI[4]
	if last != 2 {
		if last != 3 {
			t.Fatalf("wire route = %d, want 2 or 3", last)
		}
	}
	_, _, hop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
	if !found {
		t.Fatal("wire announcement omitted NEXT_HOP")
	}
	if !bytes.Equal(hop, []byte{10, 0, 0, last}) {
		t.Fatalf("wire NEXT_HOP = %x, want 10.0.0.%d", hop, last)
	}
	return last
}

func clearOutSentObserved(t *testing.T, ctx context.Context, monitor *pluginserver.MonitorClient, last byte) {
	t.Helper()
	select {
	case raw := <-monitor.EventChan:
		var event struct {
			BGP struct {
				Message struct {
					Type      string `json:"type"`
					Direction string `json:"direction"`
				} `json:"message"`
				Update struct {
					NLRI map[string][]struct {
						NextHop string   `json:"next-hop"`
						Action  string   `json:"action"`
						NLRI    []string `json:"nlri"`
					} `json:"nlri"`
				} `json:"update"`
			} `json:"bgp"`
		}
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			t.Fatal(err)
		}
		if event.BGP.Message.Type != "sent" {
			t.Fatalf("independent observer got non-SENT event: %s", raw)
		}
		if event.BGP.Message.Direction != "sent" {
			t.Fatalf("independent observer got another direction: %s", raw)
		}
		paths := event.BGP.Update.NLRI["ipv4/unicast"]
		if len(paths) != 1 {
			t.Fatalf("independent observer got %d paths: %s", len(paths), raw)
		}
		if paths[0].NextHop != fmt.Sprintf("10.0.0.%d", last) {
			t.Fatalf("independent observer next hop: %s", raw)
		}
		if paths[0].Action != "add" {
			t.Fatalf("independent observer got another action: %s", raw)
		}
		if len(paths[0].NLRI) != 1 {
			t.Fatalf("independent observer got another NLRI count: %s", raw)
		}
		if paths[0].NLRI[0] != fmt.Sprintf("192.168.0.%d/32", last) {
			t.Fatalf("independent observer got another prefix: %s", raw)
		}
	case <-ctx.Done():
		t.Fatal("independent SENT observation did not arrive", ctx.Err())
	}
	if monitor.Dropped.Load() != 0 {
		t.Fatal("independent observer lost events")
	}
}

// clearOutSentConn hides Bridger only for the IPC variant. It passively
// observes complete execute-command requests and forwards every byte unchanged.
type clearOutSentConn struct {
	net.Conn
	executed chan struct{}
	once     sync.Once
	window   []byte
}

func (conn *clearOutSentConn) Read(buf []byte) (int, error) {
	n, err := conn.Conn.Read(buf)
	conn.window = append(conn.window, buf[:n]...)
	pending := conn.window
	for len(pending) != 0 {
		advance, frame, frameErr := rpc.ScanAnswerLines(pending, false)
		if frameErr != nil {
			return n, frameErr
		}
		if advance == 0 {
			if len(pending) > rpc.MaxMessageSize {
				return n, fmt.Errorf("clear-out observer exceeded its frame bound")
			}
			break
		}
		if len(frame) > rpc.MaxMessageSize {
			return n, fmt.Errorf("clear-out observer exceeded its frame bound")
		}
		pending = pending[advance:]
		_, method, params, parseErr := rpc.ParseLine(frame)
		if parseErr != nil {
			return n, parseErr
		}
		if method != "ze-plugin-callback:execute-command" {
			continue
		}
		var input rpc.ExecuteCommandInput
		if decodeErr := json.Unmarshal(params, &input); decodeErr != nil {
			return n, decodeErr
		}
		if input.Command == "clear bgp rib out" {
			conn.once.Do(func() { close(conn.executed) })
		}
	}
	conn.window = conn.window[:copy(conn.window, pending)]
	return n, err
}
