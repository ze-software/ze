// Design: docs/architecture/api/process-protocol.md -- ordered SENT retirement.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestSentObservationOrdersCapturedDeliveryBeforeRemoval holds the REAL
// onMessageSent after it captures recipients and before proc.Deliver. A
// permanent removal must join the admitted enqueue before terminal DOWN's
// applied acknowledgement, on both SDK transports. No elapsed-time oracle.
func TestSentObservationOrdersCapturedDeliveryBeforeRemoval(t *testing.T) {
	for _, native := range []bool{true, false} {
		name := "framed-json"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			var sinkMu sync.Mutex
			inventory := make(map[uint64]bool)
			consume := func(owner uint64, kind rpc.EventKind, state rpc.SessionState) {
				sinkMu.Lock()
				defer sinkMu.Unlock()
				if kind == rpc.EventKindUpdate {
					inventory[owner] = true
				}
				if kind == rpc.EventKindState && state == rpc.SessionStateDown {
					delete(inventory, owner)
				}
			}
			srv := sentObservationServer(t, ctx, native, consume)
			proc := srv.ProcessManager().GetProcess("sent-retirement-rib")
			require.NotNil(t, proc)
			require.Equal(t, native, proc.HasStructuredHandler())
			peer := testPeerInfo()
			peer.PeerOwner = 701
			guard := &sentObservationGate{}
			sentObservationInstall(peer, guard)
			attachForDelivery(t, srv, peer, proc.Name())
			body := []byte{0, 0, 0, 0}
			update := wireu.NewWireUpdate(body, 0)
			message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionSent,
				RawBytes: body, WireUpdate: update, MessageID: 1}
			encoder := format.NewJSONEncoder("sent-retirement")
			// A legitimate negotiated SENT preceding UP must still reach routing.
			onMessageSent(srv, encoder, peer, message)
			require.NoError(t, proc.DrainEventsApplied(ctx))
			sinkMu.Lock()
			beforeUp := inventory[peer.PeerOwner]
			sinkMu.Unlock()
			require.True(t, beforeUp)
			held, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			oldLogger := logger
			captured := slog.New(&sentObservationLogHold{held: held, release: release})
			logger = func() *slog.Logger { return captured }
			defer func() { logger = oldLogger }()
			sent := make(chan struct{})
			go func() { onMessageSent(srv, encoder, peer, message); close(sent) }()
			select {
			case <-held:
			case <-ctx.Done():
				t.Fatal("real SENT did not reach captured-recipient hold")
			}
			joined := guard.seal()
			ack := make(chan struct{})
			go func() {
				<-joined
				onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonPeerRemoved)
				close(ack)
			}()
			// On the unfixed producer admission is absent: force its actual
			// removal ACK BEFORE releasing the captured late SENT delivery.
			select {
			case <-joined:
				select {
				case <-ack:
				case <-ctx.Done():
					t.Fatal("unfenced removal did not acknowledge")
				}
			default:
			}
			unblock()
			select {
			case <-sent:
			case <-ctx.Done():
				t.Fatal("captured SENT did not join")
			}
			select {
			case <-ack:
			case <-ctx.Done():
				t.Fatal("terminal DOWN did not acknowledge")
			}
			require.NoError(t, proc.DrainEventsApplied(ctx))
			sinkMu.Lock()
			resurrected := inventory[peer.PeerOwner]
			sinkMu.Unlock()
			require.False(t, resurrected, "captured SENT recreated inventory after permanent-removal ACK")
			// A callback starting after sealing must also be refused, while
			// the actual wire observation remains available to monitors.
			monitor := pluginserver.NewMonitorClient(ctx, "retired-sent", []*pluginserver.Subscription{{
				Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventUpdate),
				Direction: events.DirSent,
			}}, 8)
			srv.Monitors().Add(monitor)
			defer srv.Monitors().Remove("retired-sent")
			onMessageSent(srv, encoder, peer, message)
			require.NoError(t, proc.DrainEventsApplied(ctx))
			select {
			case payload := <-monitor.EventChan:
				require.Contains(t, payload, "sent")
			case <-ctx.Done():
				t.Fatal("retired wire observation was hidden")
			}
			sinkMu.Lock()
			late := inventory[peer.PeerOwner]
			sinkMu.Unlock()
			require.False(t, late, "sealed Peer admitted a new routing SENT")
			sibling := *peer
			sibling.PeerOwner = 702
			sentObservationInstall(&sibling, &sentObservationGate{})
			onMessageSent(srv, encoder, &sibling, message)
			require.NoError(t, proc.DrainEventsApplied(ctx))
			sinkMu.Lock()
			healthy := inventory[sibling.PeerOwner]
			sinkMu.Unlock()
			require.True(t, healthy, "same-address healthy owner lost useful publication")
			encoded, err := json.Marshal(peer)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "SentObservation", "internal callback leaked into peer JSON")
		})
	}
}

// Reflection lets this NEW regression compile before the production field
// exists: its RED is the real late delivery above, never a compiler failure.
func sentObservationInstall(peer *plugin.PeerInfo, guard *sentObservationGate) {
	field := reflect.ValueOf(peer).Elem().FieldByName("SentObservation")
	if field.IsValid() {
		field.Set(reflect.ValueOf(guard))
	}
}

type sentObservationGate struct {
	mu     sync.Mutex
	closed bool
	active int
	idle   chan struct{}
}

func (gate *sentObservationGate) BeginSentObservation() bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.closed {
		return false
	}
	gate.active++
	return true
}

func (gate *sentObservationGate) EndSentObservation() {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.active--
	if gate.closed && gate.active == 0 {
		close(gate.idle)
	}
}

func (gate *sentObservationGate) seal() <-chan struct{} {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.closed = true
	gate.idle = make(chan struct{})
	if gate.active == 0 {
		close(gate.idle)
	}
	return gate.idle
}

type sentObservationLogHold struct {
	held    chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (*sentObservationLogHold) Enabled(context.Context, slog.Level) bool { return true }
func (hold *sentObservationLogHold) Handle(_ context.Context, record slog.Record) error {
	if record.Message == "OnMessageSent" {
		hold.once.Do(func() { close(hold.held); <-hold.release })
	}
	return nil
}
func (hold *sentObservationLogHold) WithAttrs([]slog.Attr) slog.Handler { return hold }
func (hold *sentObservationLogHold) WithGroup(string) slog.Handler      { return hold }

func sentObservationServer(t *testing.T, ctx context.Context, native bool, consume func(uint64, rpc.EventKind, rpc.SessionState)) *pluginserver.Server {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "sent-retirement-rib"
	require.NoError(t, registry.Register(registry.Registration{
		Name: name,
		RunEngine: func(conn net.Conn) int {
			if !native {
				conn = struct{ net.Conn }{conn}
			}
			p := sdk.NewWithConn(name, conn)
			p.SetStartupSubscriptionsIn(bgpevents.Namespace, []string{"state", "update-sent"}, nil, "full")
			p.OnEvent(func(payload string) error {
				event, err := bgp.ParseEvent([]byte(payload))
				if err != nil {
					return err
				}
				var state rpc.SessionState
				if event.GetEventType() == rpc.EventKindState {
					if err := state.UnmarshalText([]byte(event.GetPeerState())); err != nil {
						return err
					}
				}
				consume(event.PeerOwner, event.GetEventType(), state)
				return nil
			})
			p.OnStructuredEvent(func(values []any) error {
				for _, value := range values {
					event := value.(*rpc.StructuredEvent)
					consume(event.PeerOwner, event.EventType, event.State)
				}
				return nil
			})
			if err := p.Run(ctx, &sdk.Registration{}); err != nil && ctx.Err() == nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}},
		ConfigNeedHolds: config.ConfigNeedHolds(map[string]any{})}, nil)
	require.NoError(t, err)
	srv.SetProcessSpawner(&openObserverSpawner{ctx: ctx})
	require.NoError(t, srv.StartWithContext(ctx))
	t.Cleanup(func() {
		srv.Stop()
		join, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Wait(join))
		require.NoError(t, srv.ProcessManager().Wait(join))
	})
	require.NoError(t, srv.WaitForStartupComplete(ctx))
	return srv
}
