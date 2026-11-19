// Design: docs/architecture/update-building.md -- one current admitted origin.
// Related: origin_runtime_opqueue_retirement_test.go -- real Session publication.
package reactor

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAC18HeldOriginReplayCannotOverwriteReconnectedRecipient first writes a
// useful local original, then holds its replay at the final current-selection
// callback under the original Session's writer lock. A fresh FSM-established
// Session and a genuine replacement receipt complete while that callback stays
// held. Releasing the old answer must preserve the replacement's exact wire and
// the useful local output of another admitted scope. Recording connections prove
// final-writer bytes; this fixture does not claim a TCP BGP handshake.
// MUTATION: let delayed local work use revoked receipt or destination authority;
// the old history must never reach either the original or replacement wire.
func TestAC18HeldOriginReplayCannotOverwriteReconnectedRecipient(t *testing.T) {
	f := newSelectedOriginFixture(t, false)
	work, stop := context.WithTimeout(t.Context(), 15*time.Second)
	defer stop()
	reader := f.owner.CurrentSelection
	lifetime, end := context.WithCancel(work)
	defer end()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	var holding atomic.Bool
	var calls atomic.Uint32
	owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), lifetime.Done(), nil)
	require.NoError(t, err)
	owner, err = owner.WithConnection(func(ctx context.Context, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
		answer, err := reader(ctx, input)
		if holding.Load() {
			if calls.Add(1) == 2 {
				// Preserve the exact old answer while the real issuer and
				// Session publisher advance. Cancellation cannot join this
				// callback until the test releases its completion barrier.
				close(entered)
				<-release
			}
		}
		return answer, err
	}, lifetime)
	require.NoError(t, err)
	f.owner = owner
	// The borrowed writer fixture presets a token without consuming the
	// production allocator. Number this original through the same publisher as
	// its reconnect, before any output establishes a monotonic Adj-RIB-Out cut.
	initialSession := f.peer.currentSession()
	require.NotNil(t, initialSession)
	f.peer.enterEstablished(initialSession)
	f.peer.refreshForwardFacts()
	original, value := f.issue(t, selectedOriginBuildInput(), 1)
	answer, err := f.send(t, work, original)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	oldSession, oldWire := f.peer.currentSession(), f.conn
	require.NotNil(t, oldSession)
	oldToken := oldSession.initialReplay.Load()
	require.NotZero(t, oldToken)
	before := bytes.Clone(oldWire.written())
	bodies := updateBodies(t, before)
	require.Len(t, bodies, 1)
	prefix := netip.MustParsePrefix("198.18.7.0/24")
	require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, bodies[0], false))
	hop, present := bodyPathAttr(t, bodies[0], 3)
	require.True(t, present)
	require.Equal(t, []byte{192, 0, 2, 254}, hop)
	originalRecord := originRuntimeCurrent(t, f.store, originKey(f.producer, f.source, value.originIdentity, f.peer.sourceOwner))

	other, otherWire := originRevocationScope(t, f.selectedWriterFixture, "192.0.2.81", 72)
	other.session.egressRouteFilter = other.egressGate(other.session)
	healthyInput := selectedOriginBuildInput()
	healthyInput.native = []byte{24, 198, 18, 9}
	healthyValue, err := newOriginBuildValue(healthyInput)
	require.NoError(t, err)
	healthyReceipt, err := f.store.issue(work, f.producer, f.source, healthyValue, other, f.owner)
	require.NoError(t, err)
	healthyRoute := original
	healthyRoute.KeyNLRI, healthyRoute.Revision, healthyRoute.OriginReceipt = healthyValue.native, 2, healthyReceipt
	stored := *original.Route
	stored.NLRIHex = hex.EncodeToString(healthyValue.native)
	healthyRoute.Route = &stored
	f.mu.Lock()
	f.cut = healthyRoute.Revision
	f.current[string(healthyRoute.KeyNLRI)] = healthyRoute.Revision
	f.mu.Unlock()
	healthyBatch, err := plugin.NewSelectedBatch(rpc.SelectRoutesInput{Destination: other.addrString,
		DestinationSession: 72, Routes: []rpc.SelectedRoute{healthyRoute}})
	require.NoError(t, err)
	answer, err = f.api.SelectRoutes(work, healthyBatch, f.owner)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	healthyBefore := bytes.Clone(otherWire.written())
	healthyBodies := updateBodies(t, healthyBefore)
	require.Len(t, healthyBodies, 1)
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, healthyBodies[0], false))

	replay := original
	replay.Revision = 3
	holding.Store(true)
	finished, joined := make(chan error, 1), make(chan struct{})
	go func() {
		defer close(joined)
		_, err := f.send(t, work, replay)
		finished <- err
	}()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		stop()
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Error("held old origin replay did not join cleanup")
		}
	})
	select {
	case <-entered:
	case err := <-finished:
		t.Fatalf("old replay finished before its final current check: %v", err)
	case <-work.Done():
		t.Fatal("old replay never reached its final current check")
	}
	writerUnlocked := oldSession.writeMu.TryLock()
	if writerUnlocked {
		oldSession.writeMu.Unlock()
	}
	require.False(t, writerUnlocked, "the old current answer must be held inside the final writer")

	// Reuse the existing real FSM transitions and production Peer publisher,
	// retaining the configured Peer identity while changing physical Session.
	nextSession := replacementOriginSession(t, &originRuntimeSDKFixture{writer: f.selectedWriterFixture})
	t.Cleanup(nextSession.timers.StopAll)
	f.peer.enterEstablished(nextSession)
	f.peer.refreshForwardFacts()
	require.Same(t, nextSession, f.peer.currentSession())
	require.NotSame(t, oldSession, nextSession)
	require.NotEqual(t, oldToken, nextSession.initialReplay.Load())
	require.Greater(t, nextSession.initialReplay.Load(), oldToken)
	require.NotEqual(t, oldSession.connectionID, nextSession.connectionID)
	require.NotZero(t, nextSession.initialReplay.Load())
	nextInput := selectedOriginBuildInput()
	nextInput.nextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.253"))
	replacement, nextValue := f.issue(t, nextInput, 4)
	require.Greater(t, replacement.OriginReceipt, original.OriginReceipt)
	select {
	case <-originalRecord.ctx.Done():
	case <-work.Done():
		t.Fatal("replacement admission did not revoke the original generation")
	}
	answer, err = f.send(t, work, replacement)
	require.NoError(t, err)
	require.Equal(t, rpc.SelectedWritten, answer.Results[0].Outcome)
	nextBefore := bytes.Clone(f.conn.written())
	nextBodies := updateBodies(t, nextBefore)
	require.Len(t, nextBodies, 1)
	require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, nextBodies[0], false))
	hop, present = bodyPathAttr(t, nextBodies[0], 3)
	require.True(t, present)
	require.Equal(t, []byte{192, 0, 2, 253}, hop)
	select {
	case err := <-finished:
		t.Fatalf("old callback escaped its explicit completion barrier: %v", err)
	default:
	}
	require.Equal(t, healthyBefore, otherWire.written())

	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-finished:
		require.Error(t, err, "old admitted history must not complete as current replacement output")
	case <-work.Done():
		t.Fatal("released old replay did not join")
	}
	require.Equal(t, before, oldWire.written(), "the delayed replay must not add old bytes to its previous wire")
	require.Equal(t, nextBefore, f.conn.written(), "the delayed replay must not overwrite or withdraw the new winner")
	require.Equal(t, healthyBefore, otherWire.written(), "the healthy scope must keep its exact useful original output")
	current := originRuntimeCurrent(t, f.store, originKey(f.producer, f.source, nextValue.originIdentity, f.peer.sourceOwner))
	require.Equal(t, replacement.OriginReceipt, current.id)
	require.Same(t, nextValue, current.build)
	f.store.mu.Lock()
	_, oldReceiptPresent := f.store.receipts[original.OriginReceipt]
	healthyCurrent := f.store.receipts[healthyReceipt]
	f.store.mu.Unlock()
	require.False(t, oldReceiptPresent, "the old completion must release its revoked receipt")
	require.NotNil(t, healthyCurrent)
	require.Same(t, other, healthyCurrent.peer)
}
