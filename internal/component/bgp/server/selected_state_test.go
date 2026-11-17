// Design: docs/architecture/api/process-protocol.md -- selected state projection.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type selectedStateObservation struct {
	process  string
	selected bool
	session  uint64
	replay   uint64
}

func selectedStateObservationJSON(name string, value string) (selectedStateObservation, error) {
	var event struct {
		BGP struct {
			Selected bool   `json:"selected-destination"`
			Session  uint64 `json:"destination-session,string"`
			Replay   uint64 `json:"initial-replay,string"`
		} `json:"bgp"`
	}
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		return selectedStateObservation{}, err
	}
	return selectedStateObservation{process: name, selected: event.BGP.Selected, session: event.BGP.Session, replay: event.BGP.Replay}, nil
}

func selectedStateBridgeProcess(t *testing.T, name string, observations chan<- selectedStateObservation) *process.Process {
	t.Helper()
	ready, stop := make(chan struct{}), make(chan struct{})
	require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			defer conn.Close() //nolint:errcheck // The test owns and joins this connection's lifetime.
			bridged, ok := conn.(rpc.Bridger)
			if !ok {
				return 1
			}
			bridge := bridged.Bridge()
			bridge.SetDeliverStructured(func(values []any) error {
				for _, value := range values {
					event, ok := value.(*rpc.StructuredEvent)
					if !ok {
						return errors.New("state delivery did not carry a structured event")
					}
					observations <- selectedStateObservation{process: name, selected: event.SelectedDestination,
						session: event.DestinationSession, replay: event.InitialReplay}
				}
				return nil
			})
			bridge.SetDeliverEvents(func([]string) error { return errors.New("structured delivery used text") })
			bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return nil, errors.New("this fixture accepts no engine request")
			})
			bridge.SetReady()
			close(ready)
			<-stop
			return 0
		}}))
	proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true, Encoder: plugin.EncodingJSON})
	require.NoError(t, proc.StartWithContext(t.Context()))
	require.NoError(t, proc.InitConns())
	t.Cleanup(func() {
		close(stop)
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, proc.Wait(ctx))
	})
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("internal state fixture did not activate its bridge")
	}
	return proc
}

func selectedStateWireProcess(t *testing.T, name string, observations chan<- selectedStateObservation) *process.Process {
	t.Helper()
	proc, conn := newTestProcWithConn(t, name)
	proc.SetEncoding(plugin.EncodingJSON)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			request, err := conn.ReadRequest(t.Context())
			if err != nil {
				return
			}
			events, err := rpc.ParseBatchEvents(request.Params)
			if err != nil {
				t.Errorf("state batch: %v", err)
				return
			}
			for _, event := range events {
				var value string
				if err := json.Unmarshal(event, &value); err != nil {
					t.Errorf("state event: %v", err)
					return
				}
				observation, err := selectedStateObservationJSON(name, value)
				if err != nil {
					t.Errorf("state projection: %v", err)
					return
				}
				observations <- observation
			}
			if err := conn.SendResult(t.Context(), request.ID, nil); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Logf("close state fixture: %v", err)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("state fixture reader did not stop")
		}
	})
	return proc
}

// TestSelectedDestinationDeliveryTransports proves the actual delivery loop
// distinguishes the selected owner from a same-encoding state observer, on an
// activated internal bridge and framed external-style IPC. DOWN preserves the
// ended identity without granting selected output; attached-UP seeds only owner.
func TestSelectedDestinationDeliveryTransports(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "wire", true: "bridge"}[typed], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			s := newTestServer(t)
			const owner, observer = "selected-state-owner", "selected-state-observer"
			observations := make(chan selectedStateObservation, 4)
			var processes []*process.Process
			for _, name := range []string{owner, observer} {
				if typed {
					processes = append(processes, selectedStateBridgeProcess(t, name, observations))
					continue
				}
				processes = append(processes, selectedStateWireProcess(t, name, observations))
			}
			const token = uint64(18446744073709551615)
			peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.8"), PeerAS: rpc.RemoteASOf(65001),
				DestinationSession: token, SelectedOwner: owner}
			for _, state := range []rpc.SessionState{rpc.SessionStateUp, rpc.SessionStateDown} {
				reason := rpc.ReasonAttached
				if state == rpc.SessionStateDown {
					reason = rpc.ReasonPeerRemoved
				}
				var cache formatCache
				var scratch [512]byte
				deliverPeerState(s, &peer, processes, nil, state, reason, false, &cache, &scratch)
				for range len(processes) {
					select {
					case observation := <-observations:
						require.Equal(t, token, observation.session)
						selected := state == rpc.SessionStateUp && observation.process == owner
						require.Equal(t, selected, observation.selected)
						if selected {
							require.Equal(t, token, observation.replay)
						} else {
							require.Zero(t, observation.replay)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("state delivery did not reach both recipients")
					}
				}
			}
			require.False(t, peer.SelectedDestination, "per-process projection must not mutate shared metadata")
			require.Zero(t, peer.InitialReplay)
		})
	}
}
