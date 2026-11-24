// Design: docs/architecture/api/process-protocol.md -- authenticated source leases.
package reactor

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// sourceLeaseProcessFactory owns genuine Process lifetimes with initialized
// connections. All instances use one implementation but keep configured names.
func sourceLeaseProcessFactory(t *testing.T) func(string) *process.Process {
	t.Helper()
	// The isolated reactor test package does not link the BGP composition
	// root which registers these production event names.
	require.NoError(t, events.RegisterNamespace("bgp", "update", "state", "refresh"))
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const implementation = "source-lease-test-runner"
	require.NoError(t, registry.Register(registry.Registration{Name: implementation,
		Sends: testAllBGPSendDecls(), Receives: testAllBGPReceiveDecls(),
		RunEngine: func(conn net.Conn) int {
			var input [1]byte
			if _, err := conn.Read(input[:]); err != nil {
				return 0
			}
			return 0
		}, CLIHandler: func([]string) int { return 0 },
	}))
	return func(name string) *process.Process {
		proc := process.NewProcess(plugin.PluginConfig{Name: name, Run: implementation, Internal: true})
		require.NoError(t, proc.StartWithContext(t.Context()))
		require.NoError(t, proc.InitConns())
		proc.Conn().SetBridge(proc.Bridge())
		proc.SetRegistration(&plugin.PluginRegistration{
			Sends: testAllBGPSendDecls(), Receives: testAllBGPReceiveDecls(),
			Dissemination: []rpc.DisseminationDecl{{Namespace: "bgp", Role: sourceLeaseRole,
				SourceReceive: []string{"update-received", "state"}, DestinationReceive: []string{"state"}, DestinationSend: []string{"update"}}},
		})
		t.Cleanup(func() {
			proc.Stop()
			sourceLeaseCleanup(proc)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, proc.Wait(ctx))
		})
		return proc
	}
}

func sourceLeaseTestServer(t *testing.T, procs ...*process.Process) *pluginserver.Server {
	t.Helper()
	plugins := make([]plugin.PluginConfig, 0, len(procs))
	for _, proc := range procs {
		plugins = append(plugins, proc.Config())
	}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: plugins}, nil)
	require.NoError(t, err)
	return srv
}

func sourceLeaseGrant(name string) testPeerEndpointGrant {
	return testPeerEndpointGrant{
		Name:    name,
		Receive: []string{"update-received", "state"},
	}
}

func sourceLeaseRequest(action rpc.SourceLeaseAction, receipt, peerOwner uint64) rpc.SourceLeaseInput {
	return rpc.SourceLeaseInput{Action: action, SourceReceipt: receipt, PeerOwner: peerOwner, DeadlineMS: time.Now().Add(5 * time.Second).UnixMilli()}
}

// TestSourceLeaseAuthenticatedAdapters sends both raw JSON and native values
// through the actual shared evaluator, with useful acquisition and retain/release.
func TestSourceLeaseAuthenticatedAdapters(t *testing.T) {
	proc := sourceLeaseProcessFactory(t)("rib-one")
	peer, facts := sourceReceiptFixture(t)
	peer.settings.Bus = testBusBlockForGrants(sourceLeaseGrant(proc.Name()))
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	server := sourceLeaseTestServer(t, proc)
	for _, native := range []bool{false, true} {
		for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseAcquire, rpc.SourceLeaseRetain, rpc.SourceLeaseRelease} {
			input := sourceLeaseRequest(action, live.ID(), peer.sourceOwner)
			var result any
			if native {
				result, err = sourceLeaseNative(server, t.Context(), proc, input)
			} else {
				raw, marshalErr := json.Marshal(input)
				require.NoError(t, marshalErr)
				_, err = sourceLeaseCancellation(server, proc, raw)
				require.NoError(t, err)
				result, err = sourceLeaseJSON(server, t.Context(), proc, raw)
			}
			require.NoError(t, err)
			output, valid := result.(*rpc.SourceLeaseOutput)
			require.True(t, valid)
			require.NoError(t, output.Validate(input))
		}
	}
	var nilInput *rpc.SourceLeaseInput
	_, err = sourceLeaseNative(server, t.Context(), proc, nilInput)
	require.Error(t, err)
	_, err = sourceLeaseJSON(server, t.Context(), proc, json.RawMessage(`{"action":1,"source-receipt":7,"deadline-ms":99,"peer":"192.0.2.1"}`))
	require.Error(t, err)
}

// TestSourceLeaseRequiresExactExplicitSource proves partial, derived and sibling
// process grants cannot authorize an original source's lease.
func TestSourceLeaseRequiresExactExplicitSource(t *testing.T) {
	makeProcess := sourceLeaseProcessFactory(t)
	first, second := makeProcess("rib-one"), makeProcess("rib-two")
	peer, facts := sourceReceiptFixture(t)
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	server := sourceLeaseTestServer(t, first, second)
	for _, block := range []plugin.BusBlock{
		testBusReceiveOnly(first.Name(), "state"),
		testBusNone(),
		testBusReceiveOnly(first.Name(), "update-sent", "state"),
		testBusBlockForGrants(sourceLeaseGrant(second.Name())),
	} {
		peer.settings.Bus = block
		_, err := sourceLeaseApply(server, t.Context(), first, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner))
		require.Error(t, err)
	}
	// A sibling's full grant at the same address does not authorize this exact
	// original Peer at another TCP port.
	siblingPeer := NewPeer(&PeerSettings{Address: peer.settings.Address, Port: 1179,
		Name: "sibling", Bus: testBusBlockForGrants(sourceLeaseGrant(first.Name()))})
	sibling := deliveryPeersFromSettings([]*PeerSettings{siblingPeer.settings})[0]
	require.NoError(t, server.CheckDisseminationSource(first, sibling, "bgp", sourceLeaseRole))
	peer.settings.Bus = testBusNone()
	_, err = sourceLeaseApply(server, t.Context(), first, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner))
	require.Error(t, err)
	peer.settings.Bus = testBusBlockForGrants(sourceLeaseGrant(first.Name()))
	_, err = sourceLeaseApply(server, t.Context(), first, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner))
	require.NoError(t, err)
	// Runtime absence is authoritative even though the implementation declared
	// the same role for the earlier live declaration.
	first.SetRegistration(&plugin.PluginRegistration{})
	_, err = sourceLeaseApply(server, t.Context(), first, sourceLeaseRequest(rpc.SourceLeaseRetain, live.ID(), peer.sourceOwner))
	require.Error(t, err)
	peer.settings.Bus = testBusNone()
	_, err = sourceLeaseApply(server, t.Context(), first, sourceLeaseRequest(rpc.SourceLeaseRelease, live.ID(), peer.sourceOwner))
	require.NoError(t, err, "owned cleanup survives receive grant removal")
}

// TestSourceLeaseReleaseJoinsFinalWrite proves revocation is visible while a
// real admitted final guard remains held, and success follows its completion.
func TestSourceLeaseReleaseJoinsFinalWrite(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	proc := sourceLeaseProcessFactory(t)("rib-one")
	peer, facts := sourceReceiptFixture(t)
	peer.settings.Bus = testBusBlockForGrants(sourceLeaseGrant(proc.Name()))
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	server := sourceLeaseTestServer(t, proc)
	_, err = sourceLeaseApply(server, t.Context(), proc, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner))
	require.NoError(t, err)
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	write, err := sourceReceipts.admitWrite(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	defer write.complete()
	done := make(chan error, 1)
	go func() {
		_, err := sourceLeaseApply(server, work, proc, sourceLeaseRequest(rpc.SourceLeaseRelease, live.ID(), peer.sourceOwner))
		done <- err
	}()
	select {
	case <-write.done():
	case <-work.Done():
		t.Fatal("release did not revoke its held final write")
	}
	require.Error(t, write.current())
	select {
	case err := <-done:
		t.Fatalf("release returned before final completion: %v", err)
	default:
	}
	write.complete()
	require.NoError(t, <-done)
}

// TestSourceLeaseCleanupUsesEndedIncarnation retains two same-name owners and
// proves ended-owner cleanup cannot revoke the live replacement's lease.
func TestSourceLeaseCleanupUsesEndedIncarnation(t *testing.T) {
	makeProcess := sourceLeaseProcessFactory(t)
	old, replacement := makeProcess("rib-one"), makeProcess("rib-one")
	peer, facts := sourceReceiptFixture(t)
	peer.settings.Bus = testBusBlockForGrants(sourceLeaseGrant(old.Name()))
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	server := sourceLeaseTestServer(t, old)
	for _, proc := range []*process.Process{old, replacement} {
		_, err := sourceLeaseApply(server, t.Context(), proc, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner))
		require.NoError(t, err)
	}
	oldOwner, err := old.SelectedOwner()
	require.NoError(t, err)
	newOwner, err := replacement.SelectedOwner()
	require.NoError(t, err)
	require.False(t, oldOwner.SameIncarnation(newOwner))
	old.Stop()
	sourceLeaseCleanup(old)
	_, err = sourceReceipts.admitWrite(live.ID(), oldOwner, peer.sourcePolicyEpoch.Load())
	require.Error(t, err)
	write, err := sourceReceipts.admitWrite(live.ID(), newOwner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	write.complete()
}

// FuzzDecodeSourceLease keeps arbitrary external JSON on the parser's refusal
// path without panicking or granting an unspecified lifecycle action.
func FuzzDecodeSourceLease(f *testing.F) {
	f.Add([]byte(`{"action":1,"source-receipt":7,"deadline-ms":1}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"action":255,"source-receipt":18446744073709551615}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		input, err := decodeSourceLease(raw)
		if err != nil {
			return
		}
		require.NotZero(t, input.SourceReceipt)
		require.Contains(t, []rpc.SourceLeaseAction{rpc.SourceLeaseAcquire, rpc.SourceLeaseRetain, rpc.SourceLeaseRelease}, input.Action)
	})
}

// TestSourceLeaseGrantRevocationKeepsOtherOwner proves one process losing its
// source grant cannot revoke another process's retained authority on that peer.
func TestSourceLeaseGrantRevocationKeepsOtherOwner(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	require.NoError(t, err)
	defer live.close()
	first, _ := sourceReceiptOwner(t)
	done := make(chan struct{})
	second, err := plugin.NewSelectedOwner(plugin.ProcessSender("other-owner"), done, nil)
	require.NoError(t, err)
	for _, owner := range []plugin.SelectedOwner{first, second} {
		lease, err := store.acquire(live.ID(), owner)
		require.NoError(t, err)
		require.NoError(t, lease.retain())
	}
	defer func() { require.NoError(t, store.revokeOwner(t.Context(), second)) }()
	peer.state.Store(int32(PeerStateStopped))
	live.close()
	require.NoError(t, store.revokePeerOwner(t.Context(), peer, first))
	_, err = store.admitWrite(live.ID(), first, peer.sourcePolicyEpoch.Load())
	require.Error(t, err)
	write, err := store.admitWrite(live.ID(), second, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	write.complete()
}

// TestSourceLeasePermanentRemovalCancelsAndJoins issues a real final guard and
// proves exact-peer removal revokes before joining, then leaves no history.
func TestSourceLeasePermanentRemovalCancelsAndJoins(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	peer, facts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	require.NoError(t, err)
	owner, _ := sourceReceiptOwner(t)
	_, err = store.acquire(live.ID(), owner)
	require.NoError(t, err)
	write, err := store.admitWrite(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	defer write.complete()
	peer.stopping.Store(true)
	done := make(chan error, 1)
	go func() { done <- store.revokePeers(work, peer) }()
	select {
	case <-write.done():
	case <-work.Done():
		t.Fatal("permanent source removal did not cancel its admitted guard")
	}
	select {
	case err := <-done:
		t.Fatalf("source removal returned before held final completion: %v", err)
	default:
	}
	require.Error(t, write.current())
	write.complete()
	require.NoError(t, <-done)
	store.mu.Lock()
	empty := len(store.entries) == 0
	store.mu.Unlock()
	require.True(t, empty, "completed source removal must leave no receipt tombstone")
	live.close()
}

// TestSourceLeaseProcessStopCancelsBeforeGenerationCleanup proves a held typed
// writer need not wait for the later cleanup hook to notice its owner's Stop.
func TestSourceLeaseProcessStopCancelsBeforeGenerationCleanup(t *testing.T) {
	proc := sourceLeaseProcessFactory(t)("rib-one")
	peer, facts := sourceReceiptFixture(t)
	peer.settings.Bus = testBusBlockForGrants(sourceLeaseGrant(proc.Name()))
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	_, err = sourceLeaseApply(sourceLeaseTestServer(t, proc), t.Context(), proc, sourceLeaseRequest(rpc.SourceLeaseAcquire, live.ID(), peer.sourceOwner))
	require.NoError(t, err)
	owner, err := proc.SelectedOwner()
	require.NoError(t, err)
	write, err := sourceReceipts.admitWrite(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	defer write.complete()
	work, release := write.writeContext(t.Context())
	defer release()
	proc.Stop()
	deadline, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	select {
	case <-work.Done():
	case <-deadline.Done():
		t.Fatal("owner Stop did not cancel its held writer before generation cleanup")
	}
	require.Error(t, write.current())
	write.complete()
}
