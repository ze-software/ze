// Design: docs/architecture/plugin/rib-storage-design.md -- current replay obligations.
package rr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRRCurrentReplayFailureReconcilesWithoutRestart asks one complete replay
// question: a required current query refuses before output, so ready stays owed;
// a later finite settlement reads it again and useful output reaches ready.
func TestRRCurrentReplayFailureReconcilesWithoutRestart(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	plugin := sdk.NewWithConn("rr-read-retry", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, plugin.Close()) })
	original := rrOriginalFixture(t, "192.0.2.1", 701, 41)
	recipient := rrOriginalFixture(t, "192.0.2.2", 702, 42)
	row := rpc.StoredRoute{SourcePeer: "192.0.2.1", SourceOwner: 701, SourceConnectionID: 41, SourceReceipt: 70,
		MsgID: 80, SourcePolicyEpoch: 5, Family: "ipv4/unicast", NLRIHex: "180a0000", NLRIFraming: rpc.NLRIFramingPrefixOnly, InitialUpdate: true}
	allowed := false
	ready, relayed := 0, 0
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, _ []string, _ string) (*rpc.DispatchCommandOutput, error) {
		if command == "test-current" {
			if !allowed {
				return nil, errors.New("original receive projection unavailable")
			}
			return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"answers":[true]}`)}, nil
		}
		data, err := json.Marshal(ribevents.ReplayPage{Routes: []rpc.StoredRoute{row}})
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: data}, err
	})
	bridge.SetRelayStoredRoute(func(_ context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
		require.Equal(t, uint64(702), input.DestinationOwner)
		require.Equal(t, uint64(142), input.DestinationSession)
		require.Equal(t, []rpc.StoredRoute{row}, input.Routes)
		relayed++
		return rpc.CompletedCachedWrite(input.DeadlineMS, nil), nil
	})
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if result, handled, err := rrFixtureLeaseRPC(method, params); handled {
			return result, err
		}
		var input rpc.DispatchCommandInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		require.Equal(t, rpc.MethodDispatchCommand, method)
		ready++
		return json.Marshal(rpc.DispatchCommandOutput{Status: statusDone, DestinationOwner: input.DestinationOwner, DestinationSession: input.DestinationSession, DeadlineMS: input.DeadlineMS})
	})
	bridge.SetReady()
	rr := &routeReflector{plugin: plugin, peers: make(map[peerIdentity]*peerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo), receiveGate: "test-current"}
	rrFixtureStart(t, rr, original, 70, 7, 141)
	rrFixtureStart(t, rr, recipient, 0, 7, 142)
	rr.peers[recipient].ReplayGen = 1
	target := peerTarget{identity: recipient, session: 142, generation: 1}
	rr.replayForPeer(t.Context(), target, 17, make(chan struct{}))
	require.Zero(t, relayed)
	require.Zero(t, ready)
	require.NotNil(t, rr.peers[recipient].replayDebt)
	require.NoError(t, rr.unsettledWork(), "a failed read owns no unknown writer")
	allowed = true
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, rr.settleReplays(ctx))
	require.Equal(t, 1, relayed, "useful original current route survives reconciliation")
	require.Equal(t, 1, ready)
	require.Nil(t, rr.peers[recipient].replayDebt)
	require.NoError(t, rr.unsettledWork())
}

// TestRREndedOriginalReleaseDischargesOnlyItsUnknownOutput proves the actual
// role's release path requests its original receipt after delivery ended and
// local enrollment joined. A live source or retained useful row cannot borrow
// that completion; unknown output remains owed until those conditions hold.
// The synchronous fake acknowledges only the lease protocol. Shared reactor
// tests owe the actual held-carrier/socket join beneath that receipt.
func TestRREndedOriginalReleaseDischargesOnlyItsUnknownOutput(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	plugin := sdk.NewWithConn("rr-ended-output", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, plugin.Close()) })
	var releases []rpc.SourceLeaseInput
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		var input rpc.SourceLeaseInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		if input.Action == rpc.SourceLeaseRelease {
			releases = append(releases, input)
		}
		result, handled, err := rrFixtureLeaseRPC(method, params)
		require.True(t, handled)
		return result, err
	})
	bridge.SetReady()
	source := rrOriginalFixture(t, "192.0.2.1", 701, 41)
	sibling := rrOriginalFixture(t, "192.0.2.1", 703, 43)
	rr := &routeReflector{plugin: plugin, peers: make(map[peerIdentity]*peerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	rrFixtureStart(t, rr, source, 70, 7, 141)
	rrFixtureStart(t, rr, sibling, 71, 7, 143)
	cause := errors.New("original relay cancellation has no writer receipt")
	rr.noteSourceOutputFailure(source, "stored relay", cause)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, rr.reconcileEndedOutput(ctx))
	require.Empty(t, releases, "a live original source cannot reconcile a delayed unadmitted request")
	require.ErrorContains(t, rr.unsettledWork(), cause.Error())
	rr.mu.Lock()
	rr.peers[source].Up = false
	rr.peers[source].DownSeen = true
	rr.mu.Unlock()
	key := rrFixtureInventory(t, rr, source, rpc.StoredRoute{Family: "ipv4/unicast", NLRIHex: "180a0000", SourceReceipt: 70, MsgID: 80})
	require.NoError(t, rr.reconcileEndedOutput(ctx))
	require.Empty(t, releases, "useful retained original membership cannot lose its authority")
	rr.withdrawalMu.Lock()
	info := rr.withdrawals[source.address.String()][key]
	info.Closing = true
	rr.withdrawals[source.address.String()][key] = info
	rr.withdrawalMu.Unlock()
	require.NoError(t, rr.reconcileEndedOutput(ctx))
	require.Len(t, releases, 1)
	require.Equal(t, uint64(701), releases[0].PeerOwner)
	require.Equal(t, uint64(70), releases[0].SourceReceipt)
	require.NoError(t, rr.unsettledWork())
	require.False(t, rr.peers[source].Leased)
	require.True(t, rr.peers[sibling].Leased, "same-address sibling keeps independently acquired useful authority")
	require.Equal(t, uint64(71), rr.peers[sibling].SourceReceipt)
}

// TestRRUnknownOriginalKeepsCapturedDepartureUntilRelease follows the complete
// role sequence: an unknown writer blocks DOWN before its recovery dispatch;
// exact original Release settles it; recovery still uses the originally
// captured destination and only then may cleanup purge original membership.
func TestRRUnknownOriginalKeepsCapturedDepartureUntilRelease(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	plugin := sdk.NewWithConn("rr-owed-departure", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, plugin.Close()) })
	var order []string
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if method == rpc.MethodSourceLease {
			var input rpc.SourceLeaseInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			if input.Action == rpc.SourceLeaseRelease {
				require.Equal(t, uint64(70), input.SourceReceipt)
				order = append(order, "release")
			}
			result, _, err := rrFixtureLeaseRPC(method, params)
			return result, err
		}
		require.Equal(t, rpc.MethodUpdateRoute, method)
		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		require.Equal(t, uint64(702), input.DestinationOwner)
		require.Equal(t, uint64(142), input.DestinationSession, "owed departure cannot borrow the successor destination")
		require.Equal(t, "192.0.2.1", input.Meta["recovery-source"])
		order = append(order, "original-recovery")
		return json.Marshal(rpc.UpdateRouteOutput{Withdrawn: 1})
	})
	bridge.SetReady()
	source := rrOriginalFixture(t, "192.0.2.1", 701, 41)
	recipient := rrOriginalFixture(t, "192.0.2.2", 702, 42)
	successor := rrOriginalFixture(t, "192.0.2.2", 702, 44)
	rr := &routeReflector{plugin: plugin, peers: make(map[peerIdentity]*peerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	t.Cleanup(rr.stopDepartures)
	rrFixtureStart(t, rr, source, 70, 7, 141)
	rrFixtureStart(t, rr, recipient, 0, 7, 142)
	rrFixtureInventory(t, rr, source, rpc.StoredRoute{Family: "ipv4/unicast", NLRIHex: "180a0000", SourceReceipt: 70, MsgID: 80})
	rr.noteSourceOutputFailure(source, "cached reflection", errors.New("remote cancellation has no writer completion"))
	rr.handleStateDown(source, 7)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, rr.awaitDepartures(ctx, 701))
	require.Empty(t, order, "unsettled original writer blocks departure before any output attempt")
	require.NotNil(t, rr.peers[source].departureOwed)
	rr.mu.Lock()
	rr.peers[recipient].Up = false
	rr.mu.Unlock()
	rrFixtureStart(t, rr, successor, 0, 8, 144)
	require.NoError(t, rr.settleEvents(ctx))
	require.Equal(t, []string{"release", "original-recovery"}, order)
	require.NotContains(t, rr.peers, source, "purge follows recovery and exact original Release")
	require.True(t, rr.peers[successor].Up)
	require.Equal(t, uint64(144), rr.peers[successor].DestinationSession)
	require.NoError(t, rr.unsettledWork())
}

// TestRRValidationReadFailureRetriesAtNextSettlement preserves one queued
// current path after read refusal, then requires its useful output at the next
// finite settlement. The original error cannot become process lifetime poison.
func TestRRValidationReadFailureRetriesAtNextSettlement(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	plugin := sdk.NewWithConn("rr-validation-read-retry", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, plugin.Close()) })
	source := rrOriginalFixture(t, "192.0.2.1", 701, 41)
	target := rrOriginalFixture(t, "192.0.2.2", 702, 42)
	row := rpc.StoredRoute{SourcePeer: "192.0.2.1", SourceOwner: 701, SourceConnectionID: 41, SourceReceipt: 70, MsgID: 80,
		SourcePolicyEpoch: 5, Family: "ipv4/unicast", NLRIHex: "180a0000", NLRIFraming: rpc.NLRIFramingPrefixOnly}
	var allowed atomic.Bool
	var relayed atomic.Int64
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, _ []string, _ string) (*rpc.DispatchCommandOutput, error) {
		if command == "test-current" {
			return &rpc.DispatchCommandOutput{Status: statusDone, Data: json.RawMessage(`{"answers":[true]}`)}, nil
		}
		if !allowed.Load() {
			return nil, errors.New("required current replay-path read unavailable")
		}
		data, err := json.Marshal(ribevents.ReplayPage{Routes: []rpc.StoredRoute{row}})
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: data}, err
	})
	bridge.SetRelayStoredRoute(func(_ context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
		require.Equal(t, uint64(702), input.DestinationOwner)
		require.Equal(t, uint64(142), input.DestinationSession)
		require.Equal(t, []rpc.StoredRoute{row}, input.Routes)
		relayed.Add(1)
		return rpc.CompletedCachedWrite(input.DeadlineMS, nil), nil
	})
	rrFixtureLease(bridge)
	bridge.SetReady()
	rr := &routeReflector{plugin: plugin, peers: make(map[peerIdentity]*peerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo), receiveGate: "test-current"}
	rrFixtureStart(t, rr, source, 70, 7, 141)
	rrFixtureStart(t, rr, target, 0, 7, 142)
	rr.peers[target].Families = map[family.Family]bool{family.IPv4Unicast: true}
	rrFixtureInventory(t, rr, source, row)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	stop := rr.startValidation(t.Context())
	defer stop()
	require.NoError(t, rr.dispatchText(validationEvent(t, ribevents.ValidationRoute{Peer: source.address, PeerOwner: 701, ConnectionID: 41, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/24")})))
	require.ErrorContains(t, rr.settleValidation(ctx), "required current replay-path read unavailable")
	require.Zero(t, relayed.Load())
	require.NoError(t, rr.unsettledWork())
	allowed.Store(true)
	require.NoError(t, rr.settleValidation(ctx))
	require.Equal(t, int64(1), relayed.Load(), "the useful original current path must still relay")
	rr.validation.mu.Lock()
	pending := len(rr.validation.pending)
	failed := rr.validation.failed
	rr.validation.mu.Unlock()
	require.Zero(t, pending)
	require.NoError(t, failed)
}

// TestRRRegisteredCleanupKeepsUnknownOriginalScoped drives the actual RR
// registration and applied-settlement callback on both SDK transports and all
// five accepted input combinations. A live unknown A fails settlement while
// useful independent D still forwards. After A's physical DOWN and cleanup,
// exact original Release allows the same barrier to succeed. This is role/SDK
// protocol evidence; actual held native/cached/socket join remains a shared
// reactor question, and the daemon .ci supplies real final TCP evidence.
func TestRRRegisteredCleanupKeepsUnknownOriginalScoped(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		inputs := []rrAdmissionInput{rrAdmissionText, rrAdmissionEnvelope}
		if transport == pluginengine.TransportBridge {
			inputs = append(inputs, rrAdmissionTyped)
		}
		for _, input := range inputs {
			t.Run(transport.String()+"/"+string(rune('0'+input)), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				engine := pluginengine.Start(t, transport, runRouteReflectorCallbacks)
				source := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerOwner: 701, ConnectionID: 41, SessionGeneration: 1041, DestinationSession: 141, PeerAS: rpc.RemoteASOf(65001)}
				target := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.2"), PeerOwner: 702, ConnectionID: 42, SessionGeneration: 1042, DestinationSession: 142, PeerAS: rpc.RemoteASOf(65002)}
				useful := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.3"), PeerOwner: 703, ConnectionID: 43, SessionGeneration: 1043, DestinationSession: 143, PeerAS: rpc.RemoteASOf(65003)}
				receipt := engine.IssueSource(source.Address.String(), 701)
				survivor := engine.IssueSource(useful.Address.String(), 703)
				var mu sync.Mutex
				var batches []rpc.ForwardCachedInput
				var releases []rpc.SourceLeaseInput
				engine.SetRPCHandler(func(_ context.Context, method string, params json.RawMessage) (any, error) {
					switch method {
					case rpc.MethodForwardCached:
						var batch rpc.ForwardCachedInput
						if err := json.Unmarshal(params, &batch); err != nil {
							return nil, err
						}
						mu.Lock()
						batches = append(batches, batch)
						mu.Unlock()
						if slices.Contains(batch.IDs, uint64(80)) {
							return nil, errors.New("original cached output has no writer completion")
						}
						return rpc.CompletedCachedWrite(batch.DeadlineMS, nil), nil
					case rpc.MethodSourceLease:
						var lease rpc.SourceLeaseInput
						if err := json.Unmarshal(params, &lease); err != nil {
							return nil, err
						}
						if lease.Action == rpc.SourceLeaseRelease {
							mu.Lock()
							releases = append(releases, lease)
							mu.Unlock()
						}
						return nil, pluginengine.ErrUnhandledRPC
					case rpc.MethodUpdateRoute:
						return &rpc.UpdateRouteOutput{}, nil
					case rpc.MethodReleaseCached:
						return struct{}{}, nil
					default:
						return nil, pluginengine.ErrUnhandledRPC
					}
				})
				for _, peer := range []plugin.PeerInfo{source, target, useful} {
					input.deliverState(t, ctx, engine, peer, rpc.ReasonAttached)
				}
				input.deliverUpdate(t, ctx, engine, source, receipt, 80, false, 0)
				require.ErrorContains(t, engine.SettleEvents(ctx), "original cached output has no writer completion")
				diagnostics, err := engine.DoctorCheck(ctx, replayDoctorName)
				require.NoError(t, err)
				require.NotEmpty(t, diagnostics)
				require.Equal(t, replayDoctorCode, diagnostics[0].Code)
				require.Equal(t, "error", diagnostics[0].Severity)
				require.Contains(t, diagnostics[0].Message, "owner 701 connection 41")
				mu.Lock()
				before := len(releases)
				mu.Unlock()
				require.Zero(t, before, "a live original scope cannot borrow Release as a delayed-enrollment proof")
				input.deliverUpdate(t, ctx, engine, useful, survivor, 83, false, 3)
				mu.Lock()
				recorded := slices.Clone(batches)
				mu.Unlock()
				require.Len(t, recorded, 2)
				require.Equal(t, []uint64{83}, recorded[1].IDs, "useful unrelated original source keeps its ordinary output")
				found := false
				for _, destination := range recorded[1].Targets {
					if destination.DestinationOwner == 702 {
						found = true
						require.Equal(t, uint64(142), destination.DestinationSession)
					}
				}
				require.True(t, found, "useful D still reaches the original independent recipient")
				if input == rrAdmissionTyped {
					require.NoError(t, engine.DeliverStructured([]any{&rpc.StructuredEvent{EventType: rpc.EventKindState, PeerAddress: source.Address.String(), PeerOwner: 701, ConnectionID: 41, SessionGeneration: 1041, State: rpc.SessionStateDown}}))
				} else {
					require.NoError(t, engine.Deliver(ctx, string(format.AppendStateChange(nil, &source, rpc.SessionStateDown, rpc.ReasonNotification, nil, plugin.EncodingText))))
				}
				cleanup, err := retention.EncodeCleanup(701, 41, source.Address.String(), []family.Family{family.IPv4Unicast}, false)
				require.NoError(t, err)
				envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.CleanupEventType, Payload: []byte(cleanup)})
				require.NoError(t, err)
				require.NoError(t, engine.Deliver(ctx, string(envelope)))
				require.NoError(t, engine.SettleEvents(ctx), "registered settlement must join and reconcile the ended original scope")
				diagnostics, err = engine.DoctorCheck(ctx, replayDoctorName)
				require.NoError(t, err)
				require.Empty(t, diagnostics, "the registered doctor cannot retain retired original output as process history")
				mu.Lock()
				released := slices.Clone(releases)
				mu.Unlock()
				require.NotEmpty(t, released)
				for _, lease := range released {
					require.Equal(t, uint64(701), lease.PeerOwner)
					require.Equal(t, receipt, lease.SourceReceipt)
				}
				input.deliverUpdate(t, ctx, engine, useful, survivor, 84, false, 4)
				require.NoError(t, engine.SettleEvents(ctx), "retired original debt cannot poison useful independent work")
				mu.Lock()
				last := batches[len(batches)-1]
				mu.Unlock()
				require.Equal(t, []uint64{84}, last.IDs)
			})
		}
	}
}

// TestRRReplayDoctorKeepsCurrentScopes checks the actual SDK report request
// and runtime callback: optional read recovery clears its warning while an
// unrelated required target failure stays an error under its original scope.
// The doctor read must neither issue another report nor confer completion.
func TestRRReplayDoctorKeepsCurrentScopes(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	plugin := sdk.NewWithConn("rr-current-doctor", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, plugin.Close()) })
	var reports []rpc.ReportIssueInput
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		require.Equal(t, rpc.MethodReportIssue, method)
		var issue rpc.ReportIssueInput
		if err := json.Unmarshal(params, &issue); err != nil {
			return nil, err
		}
		reports = append(reports, issue)
		return json.RawMessage(`{}`), nil
	})
	bridge.SetReady()
	original := rrOriginalFixture(t, "192.0.2.2", 702, 42)
	healthy := rrOriginalFixture(t, "192.0.2.3", 703, 43)
	rr := &routeReflector{plugin: plugin, peers: map[peerIdentity]*peerState{
		original: {Identity: original, Up: true, DestinationSession: 142, ReplayGen: 1},
		healthy:  {Identity: healthy, Up: true, DestinationSession: 143, ReplayGen: 1},
	}}
	target := peerTarget{identity: original, session: 142, generation: 1}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	rr.reportOptionalReplay(ctx, target, errors.New("optional page read refused"))
	rows, err := rr.replayDoctorCheck(replayDoctorName)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "warning", rows[0].Severity)
	require.Len(t, reports, 1)
	require.Equal(t, replayDoctorCode, reports[0].Code)
	require.Contains(t, reports[0].Subject, "owner 702 connection 42 session 142")
	rr.noteTargetOutputFailure(target, "final marker", errors.New("original writer completion unknown"))
	rr.reportOptionalReplay(ctx, target, nil)
	rows, err = rr.replayDoctorCheck(replayDoctorName)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "error", rows[0].Severity)
	require.Contains(t, rows[0].Message, "original writer completion unknown")
	require.Contains(t, rows[0].Message, "owner 702 connection 42")
	require.NotContains(t, rows[0].Message, healthy.address.String())
	require.Len(t, reports, 1, "successful read and read-only doctor do not invent another failure event")
	require.ErrorContains(t, rr.unsettledWork(), "original writer completion unknown")
	_, err = rr.replayDoctorCheck("unknown")
	require.Error(t, err)
	require.Equal(t, []string{replayDoctorCode}, replayDoctorDeclaration().Codes)
}

// TestRRHeldReadinessKeepsItsOriginalActor takes the original destination DOWN
// while its final ready call is held. Source cleanup may not erase that actor
// before the actual call returns unknown, and the named original debt must
// refuse settlement while an independent attached actor remains useful.
func TestRRHeldReadinessKeepsItsOriginalActor(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	role, start := newReplayQuestion(t, func(context.Context, []rpc.StoredRoute) error { return nil }, func(ctx context.Context) error {
		close(entered)
		select {
		case <-release:
			return errors.New("original ready writer completion unknown")
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	t.Cleanup(unblock)
	start("192.0.2.251", 173, 203)
	replayQuestionWait(t, entered, "original final readiness")
	original := rrOriginalFixture(t, "192.0.2.251", 173, 203)
	require.NoError(t, role.handleStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.251", PeerOwner: 173, ConnectionID: 203, SessionGeneration: 1203, State: rpc.SessionStateDown}))
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, role.awaitDepartures(ctx, 173))
	role.mu.RLock()
	state := role.peers[original]
	calls := uint64(0)
	if state != nil {
		calls = state.replayCalls
	}
	role.mu.RUnlock()
	require.NotNil(t, state, "normal DOWN cannot erase an admitted final marker's actor")
	require.Equal(t, uint64(1), calls)
	unblock()
	require.ErrorContains(t, role.settleReplays(ctx), "original ready writer completion unknown")
	role.mu.RLock()
	debt := role.peers[original].targetOutputDebt
	role.mu.RUnlock()
	require.ErrorContains(t, debt, "original ready writer completion unknown")
	useful := rrOriginalFixture(t, "192.0.2.252", 174, 204)
	require.NoError(t, role.handleStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.252", PeerOwner: 174, ConnectionID: 204, SessionGeneration: 1204, DestinationSession: 304, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached}))
	require.True(t, role.peers[useful].Up, "unrelated accepted state is not poisoned by the original output obligation")
}
