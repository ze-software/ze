// Design: docs/architecture/api/process-protocol.md -- accepted redistribution boundary.
package redistribute

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func bgpAcceptedFixtureEntry(t *testing.T) configredist.RouteEntry {
	t.Helper()
	return configredist.RouteEntry{Prefix: "10.9.0.0/24", Protocol: redistevents.RegisterProtocol("static"), SourceMetric: 7,
		SourceInstance: 11, Table: 90, NextHop: "192.0.2.1", HasDistanceOverride: true, DistanceOverride: 0, Metric: 99, HasMetric: true, Community: []uint32{65000<<16 | 17}}
}

func bgpAcceptedFixtureQuery(record *bgpAcceptedRecord, operation rpc.OriginIntakeOperation) rpc.OriginAdmissionLookupInput {
	return rpc.OriginAdmissionLookupInput{AdmissionID: record.id, Family: record.key.family.String(), Prefix: record.key.prefix.String(), Destination: "192.0.2.7", PeerOwner: 91, Operation: operation, DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
}

// Original preference metric and explicit zero override survive independently
// of transformed wire MED; caller buffers cannot mutate the accepted snapshot.
// MUTATION: SourceMetric taken from wire Metric makes the original metric99.
func TestBGPAcceptedSourcePreservesOriginalPreference(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	records := newBGPAcceptedRecords()
	entry := bgpAcceptedFixtureEntry(t)
	first, old, err := records.offer(family.IPv4Unicast, entry)
	require.NoError(t, err)
	require.Empty(t, old)
	entry.Community[0] = 1
	out, err := records.lookup(t.Context(), bgpAcceptedFixtureQuery(first, rpc.OriginIntakeAnnounce))
	require.NoError(t, err)
	require.Equal(t, uint32(7), out.Snapshot.SourceMetric)
	require.Equal(t, uint32(99), out.Snapshot.Metric)
	require.True(t, out.Snapshot.HasMetric)
	require.True(t, out.Snapshot.HasDistanceOverride)
	require.Zero(t, out.Snapshot.DistanceOverride)
	require.Zero(t, out.Snapshot.RuleGeneration)
	require.Equal(t, []uint32{65000<<16 | 17}, out.Snapshot.Community)
	out.Snapshot.Community[0] = 2
	again, err := records.lookup(t.Context(), bgpAcceptedFixtureQuery(first, rpc.OriginIntakeAnnounce))
	require.NoError(t, err)
	require.Equal(t, []uint32{65000<<16 | 17}, again.Snapshot.Community)
}

// Same-prefix source instances remain independent. An unknown removal creates
// no record; withdrawing one genuine source cannot retire the useful sibling.
// MUTATION: omitting instance/table from the accepted key collapses the sources.
func TestBGPAcceptedWithdrawalKeepsIndependentSource(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	records := newBGPAcceptedRecords()
	entry := bgpAcceptedFixtureEntry(t)
	first, _, err := records.offer(family.IPv4Unicast, entry)
	require.NoError(t, err)
	otherEntry := entry
	otherEntry.SourceInstance++
	other, _, err := records.offer(family.IPv4Unicast, otherEntry)
	require.NoError(t, err)
	removal := configredist.RouteRemoval{Prefix: entry.Prefix, Protocol: entry.Protocol, SourceInstance: entry.SourceInstance, Table: entry.Table}
	retiring, err := records.retiring(family.IPv4Unicast, removal)
	require.NoError(t, err)
	require.Equal(t, []*bgpAcceptedRecord{first}, retiring)
	_, err = records.lookup(t.Context(), bgpAcceptedFixtureQuery(first, rpc.OriginIntakeAnnounce))
	require.Error(t, err)
	_, err = records.lookup(t.Context(), bgpAcceptedFixtureQuery(first, rpc.OriginIntakeWithdraw))
	require.NoError(t, err)
	records.withdrawn(first)
	_, err = records.lookup(t.Context(), bgpAcceptedFixtureQuery(other, rpc.OriginIntakeAnnounce))
	require.NoError(t, err)
	before := records.next
	removal.SourceInstance += 2
	missing, err := records.retiring(family.IPv4Unicast, removal)
	require.NoError(t, err)
	require.Empty(t, missing)
	require.Equal(t, before, records.next)
	require.Len(t, records.ids, 1)
}

// Replacing accepted data retires its old version without borrowing the new
// snapshot; a targeted replay retains the original all-peer record and ID.
// MUTATION: reusing a changed accepted ID lets old in-flight commands adopt new data.
func TestBGPAcceptedVersionAndReplayStayExact(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	records := newBGPAcceptedRecords()
	entry := bgpAcceptedFixtureEntry(t)
	first, _, err := records.offer(family.IPv4Unicast, entry)
	require.NoError(t, err)
	targeted := entry
	targeted.Peer = "192.0.2.7"
	replay, old, err := records.offer(family.IPv4Unicast, targeted)
	require.NoError(t, err)
	require.Same(t, first, replay)
	require.Empty(t, old)
	changed := entry
	changed.Metric = 100
	changed.AdmissionGeneration = 1
	latest, old, err := records.offer(family.IPv4Unicast, changed)
	require.NoError(t, err)
	require.NotEqual(t, first.id, latest.id)
	require.Equal(t, []*bgpAcceptedRecord{first}, old)
	_, err = records.lookup(t.Context(), bgpAcceptedFixtureQuery(first, rpc.OriginIntakeAnnounce))
	require.Error(t, err)
	out, err := records.lookup(t.Context(), bgpAcceptedFixtureQuery(first, rpc.OriginIntakeWithdraw))
	require.NoError(t, err)
	require.Equal(t, uint32(99), out.Snapshot.Metric)
	records.finish(old)
	_, err = records.lookup(t.Context(), bgpAcceptedFixtureQuery(first, rpc.OriginIntakeWithdraw))
	require.Error(t, err)
	out, err = records.lookup(t.Context(), bgpAcceptedFixtureQuery(latest, rpc.OriginIntakeAnnounce))
	require.NoError(t, err)
	require.Equal(t, uint32(100), out.Snapshot.Metric)
}

// Actual SDK startup declares the callback before Started. A blocked route RPC
// obtains the original snapshot through the independent typed/framed callback,
// and a retained-loser result raises no false zero-peer failure.
func TestBGPAcceptedActualSDKRetainedResult(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			client, server := net.Pipe()
			mux := rpc.NewMuxConn(rpc.NewConn(server, server))
			connection := ipc.NewMuxPluginConn(mux)
			var bridge *rpc.DirectBridge
			var clientConn net.Conn = client
			if typed {
				bridge = rpc.NewDirectBridge()
				clientConn = rpc.NewBridgedConn(client, bridge)
				connection.SetBridge(bridge)
			}
			var mu sync.Mutex
			var snapshots []rpc.OriginAdmissionSnapshot
			var reports int
			var declaration bool
			handle := func(method string, raw json.RawMessage) (json.RawMessage, error) {
				if method == rpc.MethodReportIssue {
					mu.Lock()
					reports++
					mu.Unlock()
					return json.RawMessage(`{}`), nil
				}
				if method != rpc.MethodUpdateRoute {
					return nil, errors.New("unexpected runtime method")
				}
				var input rpc.UpdateRouteInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, err
				}
				if input.OriginAdmissionID == 0 {
					return nil, errors.New("BGP adapter lost its original accepted ID")
				}
				operation := rpc.OriginIntakeAnnounce
				if strings.Contains(input.Command, " del ") {
					operation = rpc.OriginIntakeWithdraw
				}
				query := rpc.OriginAdmissionLookupInput{AdmissionID: input.OriginAdmissionID, Family: "ipv4/unicast", Prefix: "10.9.0.0/24", Destination: "192.0.2.7", PeerOwner: 91, Operation: operation}
				output, err := connection.SendOriginAdmissionLookup(ctx, query)
				if err != nil {
					return nil, err
				}
				mu.Lock()
				snapshots = append(snapshots, output.Snapshot)
				mu.Unlock()
				result := rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 9, 0}, Destination: "192.0.2.7", PeerOwner: 91, Generation: 1, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}
				if operation == rpc.OriginIntakeWithdraw {
					result = rpc.UpdateRouteOutput{Withdrawn: 1}
				}
				return json.Marshal(result)
			}
			if bridge != nil {
				bridge.SetDispatchRPC(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
					return handle(method, raw)
				})
			}
			engineDone := make(chan error, 1)
			go func() {
				for request := range mux.Requests() {
					switch request.Method {
					case rpc.MethodDeclareRegistration:
						var input rpc.DeclareRegistrationInput
						if err := json.Unmarshal(request.Params, &input); err != nil {
							engineDone <- err
							return
						}
						mu.Lock()
						declaration = input.OriginAdmissionLookup
						mu.Unlock()
						if err := mux.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}); err != nil {
							engineDone <- err
							return
						}
						if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", map[string]any{}); err != nil {
							engineDone <- err
							return
						}
					case "ze-plugin-engine:declare-capabilities":
						if err := mux.SendResult(ctx, request.ID, map[string]any{}); err != nil {
							engineDone <- err
							return
						}
						if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", map[string]any{}); err != nil {
							engineDone <- err
							return
						}
					case "ze-plugin-engine:ready":
						if err := mux.SendResult(ctx, request.ID, map[string]any{}); err != nil {
							engineDone <- err
							return
						}
					default:
						output, err := handle(request.Method, request.Params)
						if err != nil {
							if err = mux.SendError(ctx, request.ID, err.Error()); err != nil {
								engineDone <- err
								return
							}
						} else if err = mux.SendResult(ctx, request.ID, output); err != nil {
							engineDone <- err
							return
						}
					}
				}
				engineDone <- nil
			}()
			p := sdk.NewWithConn("accepted-bgp-adapter-test", clientConn)
			consumer := NewBGPConsumer(p)
			started := make(chan struct{})
			p.OnStarted(func(context.Context) error { close(started); return nil })
			pluginDone := make(chan error, 1)
			go func() { pluginDone <- p.Run(ctx, &sdk.Registration{}) }()
			t.Cleanup(func() {
				cancel()
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				if err := mux.Close(); err != nil {
					t.Error(err)
				}
				cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				select {
				case err := <-pluginDone:
					if err != nil {
						t.Error(err)
					}
				case <-cleanup.Done():
					t.Error("accepted adapter SDK did not join")
				}
				select {
				case <-engineDone:
				case <-cleanup.Done():
					t.Error("accepted adapter engine did not join")
				}
			})
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("actual SDK did not reach Started")
			}
			entry := bgpAcceptedFixtureEntry(t)
			consumer.InjectRoute(ctx, family.IPv4Unicast, entry)
			mu.Lock()
			declared := declaration
			captured := append([]rpc.OriginAdmissionSnapshot(nil), snapshots...)
			raised := reports
			mu.Unlock()
			require.True(t, declared)
			require.Len(t, captured, 1)
			snapshot := captured[0]
			require.Zero(t, raised)
			require.Equal(t, uint32(7), snapshot.SourceMetric)
			require.Equal(t, uint32(99), snapshot.Metric)
			require.True(t, snapshot.HasDistanceOverride)
			require.Zero(t, snapshot.DistanceOverride)
			require.NoError(t, consumer.WithdrawRoute(ctx, family.IPv4Unicast, configredist.RouteRemoval{Prefix: entry.Prefix, Protocol: entry.Protocol, SourceInstance: entry.SourceInstance, Table: entry.Table}), "the confirmed withdrawal answers nil")
			mu.Lock()
			captured = append([]rpc.OriginAdmissionSnapshot(nil), snapshots...)
			raised = reports
			mu.Unlock()
			require.Len(t, captured, 2)
			require.Zero(t, raised)
			require.Empty(t, consumer.accepted.ids)
		})
	}
}

// Existing formatter/report tests supply the original source at the newly
// explicit boundary; their original wire assertions are unchanged.
func bgpAcceptedFixtureProtocol(t *testing.T) redistevents.ProtocolID {
	t.Helper()
	id, known := redistevents.ProtocolIDOf("static")
	if known {
		return id
	}
	t.Cleanup(redistevents.PreserveForTest())
	return redistevents.RegisterProtocol("static")
}

func bgpAcceptedRemoval(t *testing.T, prefix string) configredist.RouteRemoval {
	t.Helper()
	return configredist.RouteRemoval{Prefix: prefix, Protocol: bgpAcceptedFixtureProtocol(t)}
}

func seedBGPAcceptedRemoval(t *testing.T, consumer *BGPConsumer, fam family.Family, removal configredist.RouteRemoval) {
	t.Helper()
	_, _, err := consumer.accepted.offer(fam, configredist.RouteEntry{Prefix: removal.Prefix, Protocol: removal.Protocol, SourceInstance: removal.SourceInstance, Table: removal.Table})
	require.NoError(t, err)
}
