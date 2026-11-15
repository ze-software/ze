// Design: docs/architecture/api/process-protocol.md -- stage 5 startup subscriptions, before any event flows
// Related: startup.go -- engineStartupSink.onReady registers them before the Stage 5 OK
// Related: dispatch.go -- deliverEvent, the core of emit-event on both transports
package server

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// windowArrival is one notice a window consumer received, and whether its
// OnStarted had returned when it arrived.
type windowArrival struct {
	event          string
	afterOnStarted bool
}

// windowEmit is what the emit a window consumer made inside OnStarted answered.
type windowEmit struct {
	delivered int
	err       error
}

// windowNotice is the irr/dataset-stored payload the window tests send, as
// the irr plugin's emitNotice marshals it.
func windowNotice(t *testing.T) string {
	t.Helper()
	payload, err := json.Marshal(store.DatasetNotice{
		ASSet: "AS-WINDOW", Server: "whois.radb.net", Generation: 1, Digest: strings.Repeat("ab", 32),
	})
	require.NoError(t, err)
	return string(payload)
}

// runWindowConsumer runs an SDK consumer of irr/dataset-stored over conn,
// subscribed the way bgp-filter-irr and firewall-irr subscribe: through
// AddStartupSubscriptionsIn, carried by the Stage 5 ready request. Its
// OnStarted calls emit first, which sends one notice from outside the
// consumer, records what emit answered, and only then reaches the point
// where a real consumer reads the store. Every notice OnEvent receives goes
// to arrivals, marked with whether OnStarted had returned.
func runWindowConsumer(name string, conn net.Conn, emit func(context.Context) (int, error),
	emitted chan<- windowEmit, arrivals chan<- windowArrival,
) int {
	p := sdk.NewWithConn(name, conn)
	var returned atomic.Bool
	p.AddStartupSubscriptionsIn(store.NoticeNamespace, []string{store.EventDatasetStored})
	p.OnEvent(func(event string) error {
		arrivals <- windowArrival{event: event, afterOnStarted: returned.Load()}
		return nil
	})
	p.OnStarted(func(ctx context.Context) error {
		delivered, err := emit(ctx)
		emitted <- windowEmit{delivered: delivered, err: err}
		// A real consumer asks for its first store read here (readAll in
		// bgp-filter-irr and in firewall-irr): the notice above was sent
		// before it.
		returned.Store(true)
		return nil
	})
	if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
		return 1
	}
	return 0
}

// awaitWindow returns what the consumer's emit answered and the first notice
// it received, failing the test when either does not come.
func awaitWindow(t *testing.T, emitted <-chan windowEmit, arrivals <-chan windowArrival) windowArrival {
	t.Helper()
	var result windowEmit
	select {
	case result = <-emitted:
	case <-time.After(10 * time.Second):
		t.Fatal("the consumer's OnStarted never ran its emit")
	}
	require.NoError(t, result.err, "emit-event refused the notice")
	// Red: remove the registerSubscriptions calls from engineStartupSink.onReady.
	require.Equal(t, 1, result.delivered,
		"deliverEvent matched no subscriber while the consumer ran OnStarted: the stage 5 subscription was not live yet")
	select {
	case arrival := <-arrivals:
		notice, err := store.DecodeDatasetNotice(arrival.event)
		require.NoError(t, err, "the consumer received %q", arrival.event)
		assert.Equal(t, "AS-WINDOW", notice.ASSet)
		assert.Equal(t, uint32(1), notice.Generation)
		return arrival
	case <-time.After(10 * time.Second):
		t.Fatal("a notice sent between the stage 5 subscription and the OnStarted read never reached the consumer")
	}
	return windowArrival{}
}

// TestStartupSubscriptionLiveBeforeOnStartedRead proves spec assumption A-3
// (plan/release/fixes/spec-fetched-data-survives-restart.md): a consumer's
// stage 5 subscription is live before its OnStarted read, so an IRR
// dataset-stored notice sent in that window is not lost, on both transports.
//
// VALIDATES: engineStartupSink.onReady registers the ready request's
// subscriptions before the Stage 5 OK, and the SDK runs OnStarted only after
// that OK, so a notice emitted from inside OnStarted, before the read, is
// matched by deliverEvent and reaches the consumer's OnEvent. Over the socket
// it waits in the connection until OnStarted returns and the event loop
// serves it. Over DirectBridge the SDK set the bridge ready before OnStarted,
// so the engine's delivery goroutine hands it to OnEvent at once, possibly
// while OnStarted still runs: both IRR consumers' notice handlers only queue
// work for their worker, which is safe either way.
// PREVENTS: a startup subscription registered after the OK, or a consumer
// event loop that drops what arrived before it ran. Either loses the first
// notice in silence, and the consumer then enforces the copy it read until
// the next notice's generation gap makes it read again.
//
// Method: "direct" starts an owner and a consumer as internal plugins through
// runPluginPhase, the daemon's startup path, and the owner sends the notice
// with the SDK's EmitEvent while the consumer is inside OnStarted. "socket"
// runs the consumer over a pipe with no bridge, as a forked plugin talks,
// through the real handshake, and sends the notice through emitEvent, the
// handler of the emit-event RPC.
func TestStartupSubscriptionLiveBeforeOnStartedRead(t *testing.T) {
	const ownerName = "a3-window-owner"
	const consumerName = "a3-window-consumer"
	payload := windowNotice(t)

	t.Run("direct", func(t *testing.T) {
		snap := registry.Snapshot()
		registry.Reset()
		t.Cleanup(func() { registry.Restore(snap) })

		var owner atomic.Pointer[sdk.Plugin]
		ownerStarted := make(chan struct{})
		registerLifecyclePlugin(t, ownerName, nil, func(conn net.Conn) int {
			p := sdk.NewWithConn(ownerName, conn)
			owner.Store(p)
			p.OnStarted(func(context.Context) error {
				close(ownerStarted)
				return nil
			})
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		})
		emitted := make(chan windowEmit, 1)
		arrivals := make(chan windowArrival, 4)
		emit := func(ctx context.Context) (int, error) {
			select {
			case <-ownerStarted:
			case <-time.After(10 * time.Second):
				return 0, context.DeadlineExceeded
			}
			return owner.Load().EmitEvent(ctx, store.NoticeNamespace, store.EventDatasetStored, "", "", payload)
		}
		registerLifecyclePlugin(t, consumerName, nil, func(conn net.Conn) int {
			return runWindowConsumer(consumerName, conn, emit, emitted, arrivals)
		})

		s, _ := newLifecycleStartupServer(t)
		require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
			{Name: ownerName, Internal: true, Encoder: plugin.EncodingJSON},
			{Name: consumerName, Internal: true, Encoder: plugin.EncodingJSON},
		}))
		awaitWindow(t, emitted, arrivals)
	})

	t.Run("socket", func(t *testing.T) {
		s := &Server{
			subscriptions:     newSubscriptionManager(),
			engineSubscribers: newEngineEventSubscribers(),
			dispatcher:        NewDispatcher(),
			registry:          plugin.NewPluginRegistry(),
			capInjector:       plugin.NewCapabilityInjector(),
		}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		t.Cleanup(s.cancel)

		pluginSide, engineSide := net.Pipe()
		proc := process.NewProcess(plugin.PluginConfig{Name: consumerName})
		proc.SetConn(plugipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))))
		proc.SetRunning(true)
		proc.StartDelivery(s.ctx)
		served := make(chan struct{})
		go func() {
			defer close(served)
			s.runUnbarrieredStartupHandshake(s.ctx, proc)
			if proc.Stage() >= plugin.StageRunning {
				s.handleSingleProcessCommandsRPC(proc)
			}
		}()

		emitter := process.NewProcess(plugin.PluginConfig{Name: ownerName})
		emit := func(context.Context) (int, error) {
			params, err := json.Marshal(rpc.EmitEventInput{
				Namespace: store.NoticeNamespace, EventType: store.EventDatasetStored, Event: payload,
			})
			if err != nil {
				return 0, err
			}
			out, err := s.emitEvent(emitter, params)
			if err != nil {
				return 0, err
			}
			return out.Delivered, nil
		}
		emitted := make(chan windowEmit, 1)
		arrivals := make(chan windowArrival, 4)
		consumerDone := make(chan int, 1)
		go func() { consumerDone <- runWindowConsumer(consumerName, pluginSide, emit, emitted, arrivals) }()
		t.Cleanup(func() {
			assert.NoError(t, pluginSide.Close())
			assert.NoError(t, engineSide.Close())
			select {
			case <-served:
			case <-time.After(5 * time.Second):
				t.Error("the engine side of the session did not end")
			}
			select {
			case <-consumerDone:
			case <-time.After(5 * time.Second):
				t.Error("the consumer's Run did not return")
			}
		})

		arrival := awaitWindow(t, emitted, arrivals)
		// Red: an SDK that serves its connection's requests while OnStarted
		// still runs.
		assert.True(t, arrival.afterOnStarted,
			"over the socket the notice reached OnEvent before OnStarted returned: it was not held for the event loop")
	})
}
