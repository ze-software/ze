// Design: docs/architecture/api/process-protocol.md -- exact recipient denial.
// Related: selected_state_test.go -- actual bridge and framed process helpers.
package server

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type updateSendObservation struct {
	process    string
	raw        string
	denied     bool
	owner      uint64
	connection uint64
	session    uint64
	replay     uint64
	generation uint64
	roles      []string
}

// updateSendProcess reuses the activated bridge and framed IPC fixtures. Each
// configured alias publishes the actual runtime claim read by the projection.
func updateSendProcess(t *testing.T, name, transport string, claimant bool, observations chan<- updateSendObservation) *process.Process {
	t.Helper()
	var proc *process.Process
	if transport == "typed" {
		proc = selectedStateBridgeProcess(t, name, make(chan selectedStateObservation, 1))
		proc.Bridge().SetDeliverStructured(func(values []any) error {
			for _, value := range values {
				event, ok := value.(*rpc.StructuredEvent)
				if !ok {
					return fmt.Errorf("denial delivery carries %T instead of structured state", value)
				}
				observations <- updateSendObservation{process: name, denied: event.UpdateSendDenied,
					owner: event.PeerOwner, connection: event.ConnectionID, session: event.DestinationSession,
					replay: event.InitialReplay, generation: event.SessionGeneration, roles: event.UnheldRoles}
			}
			return nil
		})
	}
	if transport != "typed" {
		framed, conn := newTestProcWithConn(t, name)
		proc = framed
		proc.SetEncoding(transport)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				request, err := conn.ReadRequest(t.Context())
				if err != nil {
					return
				}
				batch, err := rpc.ParseBatchEvents(request.Params)
				if err != nil {
					t.Errorf("decode state batch: %v", err)
					return
				}
				for _, encoded := range batch {
					var raw string
					if err := json.Unmarshal(encoded, &raw); err != nil {
						t.Errorf("decode state event: %v", err)
						return
					}
					observations <- updateSendObservation{process: name, raw: raw}
				}
				if err := conn.SendResult(t.Context(), request.ID, nil); err != nil {
					return
				}
			}
		}()
		t.Cleanup(func() {
			if err := conn.Close(); err != nil {
				t.Logf("close denial fixture: %v", err)
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("denial fixture reader did not stop")
			}
		})
	}
	registration := &plugin.PluginRegistration{Name: "bgp-rs", Done: true}
	if claimant {
		registration.Claims = []string{"bgp-peer-up-replay"}
	}
	proc.SetRegistration(registration)
	require.Equal(t, registration, proc.Registration())
	return proc
}

func updateSendObservationJSON(t *testing.T, observation updateSendObservation) updateSendObservation {
	t.Helper()
	event, err := bgp.ParseEvent([]byte(observation.raw))
	require.NoError(t, err)
	observation.denied = event.UpdateSendDenied
	observation.owner = event.PeerOwner
	observation.connection = event.ConnectionID
	observation.session = event.DestinationSession
	observation.replay = event.InitialReplay
	observation.generation = event.SessionGeneration
	observation.roles = event.UnheldRoles
	return observation
}

// TestUpdateSendDeniedDeliveryTransports delivers the same captured physical
// peer to permitted and receive-only replay aliases sharing one encoding, plus
// an unrelated observer. Denial must be exact on typed, native and JSON rails,
// while DOWN, monitors and uncaptured synthetic observations remain unchanged.
// MUTATION: reuse the denied recipient's formatted cache entry for its sibling,
// omit runtime alias claims, or drop the generic JSON field; the matrix differs.
func TestUpdateSendDeniedDeliveryTransports(t *testing.T) {
	for _, transport := range []string{"typed", plugin.EncodingText, plugin.EncodingJSON} {
		t.Run(transport, func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			s := newTestServer(t)
			observations := make(chan updateSendObservation, 8)
			const permitted, denied, observer = "permit-alias", "source-alias", "passive-alias"
			procs := []*process.Process{
				updateSendProcess(t, denied, transport, true, observations),
				updateSendProcess(t, permitted, transport, true, observations),
				updateSendProcess(t, observer, transport, false, observations),
			}
			require.Contains(t, procs[0].Registration().Claims, "bgp-peer-up-replay")
			require.Contains(t, procs[1].Registration().Claims, "bgp-peer-up-replay")
			monitor := pluginserver.NewMonitorClient(t.Context(), "denial-monitor", []*pluginserver.Subscription{{
				Namespace: events.LookupNamespaceID(bgpevents.Namespace), EventType: events.LookupEventTypeID(bgpevents.EventState),
				Direction: events.DirUnspecified,
			}}, 8)
			s.Monitors().Add(monitor)
			t.Cleanup(func() { s.Monitors().Remove("denial-monitor") })
			for _, scenario := range []struct {
				name    string
				state   rpc.SessionState
				reason  string
				grants  []string
				replay  uint64
				denials []string
			}{
				{"actual-up", rpc.SessionStateUp, "", []string{permitted}, 1<<53 + 66, []string{denied}},
				{"attached-up", rpc.SessionStateUp, rpc.ReasonAttached, []string{permitted}, 0, []string{denied}},
				{"known-no-grants", rpc.SessionStateUp, "", []string{}, 1<<53 + 66, []string{denied, permitted}},
				{"down", rpc.SessionStateDown, rpc.ReasonPeerRemoved, []string{permitted}, 0, nil},
				{"uncaptured-synthetic", rpc.SessionStateUp, "", nil, 1<<53 + 66, nil},
			} {
				t.Run(scenario.name, func(t *testing.T) {
					peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.81"), PeerAS: rpc.RemoteASOf(65001),
						PeerOwner: 1<<53 + 11, ConnectionID: 1<<53 + 22, DestinationSession: 1<<53 + 33,
						SessionGeneration: 1<<53 + 44, InitialReplay: scenario.replay, UpdateSendGrants: scenario.grants}
					roles := []string{"unrelated-collective-role"}
					onPeerStateChangeTo(s, &peer, scenario.state, scenario.reason, procs, roles)
					seen := make(map[string]bool)
					for range procs {
						var got updateSendObservation
						select {
						case got = <-observations:
						case <-time.After(5 * time.Second):
							t.Fatal("state did not reach every actual recipient")
						}
						require.False(t, seen[got.process])
						seen[got.process] = true
						wantDenied := slices.Contains(scenario.denials, got.process)
						if transport == plugin.EncodingText {
							require.Equal(t, wantDenied, strings.Contains(got.raw, " update-send-denied true"))
							if !wantDenied {
								require.NotContains(t, got.raw, "update-send-denied")
							}
							for _, token := range []string{
								fmt.Sprintf("peer-owner %d", peer.PeerOwner), fmt.Sprintf("connection-id %d", peer.ConnectionID),
								fmt.Sprintf("destination-session %d", peer.DestinationSession), fmt.Sprintf("session-generation %d", peer.SessionGeneration),
							} {
								require.Contains(t, got.raw, token)
							}
							if scenario.replay != 0 {
								require.Contains(t, got.raw, fmt.Sprintf("initial-replay %d", scenario.replay))
							}
							if scenario.replay == 0 {
								require.NotContains(t, got.raw, "initial-replay")
							}
							continue
						}
						if transport == plugin.EncodingJSON {
							got = updateSendObservationJSON(t, got)
							require.Equal(t, wantDenied, strings.Contains(got.raw, `"update-send-denied":true`))
							if !wantDenied {
								require.NotContains(t, got.raw, "update-send-denied")
							}
						}
						require.Equal(t, wantDenied, got.denied)
						require.Equal(t, peer.PeerOwner, got.owner)
						require.Equal(t, peer.ConnectionID, got.connection)
						require.Equal(t, peer.DestinationSession, got.session)
						require.Equal(t, peer.InitialReplay, got.replay)
						require.Equal(t, peer.SessionGeneration, got.generation)
						require.Equal(t, roles, got.roles)
					}
					require.Len(t, seen, len(procs))
					select {
					case raw := <-monitor.EventChan:
						got := updateSendObservationJSON(t, updateSendObservation{raw: raw})
						require.False(t, got.denied)
						require.NotContains(t, raw, "update-send-denied")
						require.Equal(t, peer.PeerOwner, got.owner)
						require.Equal(t, peer.ConnectionID, got.connection)
						require.Equal(t, peer.DestinationSession, got.session)
						require.Equal(t, peer.InitialReplay, got.replay)
						require.Equal(t, peer.SessionGeneration, got.generation)
						require.Equal(t, roles, got.roles)
					case <-time.After(5 * time.Second):
						t.Fatal("monitor missed unprojected physical state")
					}
					require.False(t, peer.UpdateSendDenied, "recipient projection must not mutate shared peer facts")
				})
			}
		})
	}
}

// TestUpdateSendDeniedStructuredPoolReset requires the denial advisory to end
// with its actual event rather than surviving into the next pooled delivery.
func TestUpdateSendDeniedStructuredPoolReset(t *testing.T) {
	event := rpc.GetStructuredEvent()
	event.UpdateSendDenied = true
	rpc.PutStructuredEvent(event)
	require.False(t, event.UpdateSendDenied)
}
