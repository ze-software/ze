// Design: docs/architecture/api/process-protocol.md -- granular origin revocation.
// Related: origin_receipt_lifecycle_test.go -- existing writer and resource controls.
package reactor

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/selector"
)

// originIssuePauseContext stops the second process-lifetime check: issue has
// checked the producer, concrete grant and stopping flag and is inside the
// ordinary snapshot's ExportKind, before taking the issuer commit mutex.
// It changes no lifetime answer. Safe for concurrent use; arm only while idle.
type originIssuePauseContext struct {
	context.Context
	armed       atomic.Bool
	calls       atomic.Int32
	entered     chan struct{}
	resume      chan struct{}
	releaseOnce sync.Once
}

func (pause *originIssuePauseContext) Err() error {
	if pause.armed.Load() {
		if pause.calls.Add(1) == 2 {
			if pause.armed.CompareAndSwap(true, false) {
				close(pause.entered)
				select {
				case <-pause.resume:
				case <-pause.Context.Done():
				}
			}
		}
	}
	return pause.Context.Err()
}

func (pause *originIssuePauseContext) release() {
	pause.releaseOnce.Do(func() { close(pause.resume) })
}

// Done is evaluated only when the real final-write waiter reaches its blocking
// select. This observes actual wait entry, never scheduling delay or elapsed time.
type originRevocationWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (wait *originRevocationWaitContext) Done() <-chan struct{} {
	wait.once.Do(func() { close(wait.entered) })
	return wait.Context.Done()
}

func originRevocationPublishGraph(t *testing.T, writer *selectedWriterFixture) {
	t.Helper()
	writer.api.r.mu.RLock()
	settings := make([]*PeerSettings, 0, len(writer.api.r.peers))
	for _, p := range writer.api.r.peers {
		settings = append(settings, p.settingsSnapshot())
	}
	writer.api.r.mu.RUnlock()
	peers := append([]pluginserver.DeliveryPeer{
		testDeliveryPeer("192.0.2.79", "", testBusSourceLease(selectedWriterProcess)),
	}, deliveryPeersFromSettings(settings)...)
	endpoints := buildTestEndpointInputsForPeers(peers, writer.api.r.api, nil)
	require.NoError(t, writer.api.r.api.UpdateDeliveryGraphWithEndpoints("bgp", peers, endpoints))
}

// originRevocationScope supplies a distinct original Peer and an actual
// Established recording Session using the same concrete declaration resolver.
func originRevocationScope(t *testing.T, writer *selectedWriterFixture, address string, token uint64) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, address, "absent")
	peer.settings.LocalAddress = writer.peer.settings.LocalAddress
	peer.settings.ExportPolicy = writer.peer.settings.ExportPolicy
	peer.settings.Bus = writer.peer.settings.Bus.Clone()
	peer.sendCtx.Store(writer.peer.sendCtx.Load())
	peer.sendCtxID = writer.peer.sendCtxID
	peer.session.sendCtxID = writer.peer.session.sendCtxID
	peer.session.initialReplay.Store(token)
	peer.session.egressFinalAccepts = func(kind filterapi.ExportKind) bool {
		return peer.forwardFacts().exportPolicy.AcceptsExportAtEnd(true, kind)
	}
	peer.reactor = writer.api.r
	writer.api.r.mu.Lock()
	writer.api.r.peers[peer.Settings().PeerKey()] = peer
	writer.api.r.mu.Unlock()
	originRevocationPublishGraph(t, writer)
	writer.api.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: peer.Settings().PeerKey()}, 4096)
	peer.refreshForwardFacts()
	require.NotNil(t, peer.selectedOwnerSnapshot())
	require.NotEqual(t, writer.peer.sourceOwner, peer.sourceOwner)
	return peer, conn
}

// This inactive foundation has no origin final writer yet. Its healthy receipt
// stays usable while the existing authorized local announce rail supplies actual
// useful socket output as an independent scope-isolation control.
func originRevocationHealthyOutput(t *testing.T, writer *selectedWriterFixture, peer *Peer, conn *recordingConn, write originWrite, producer originProducer) {
	t.Helper()
	require.NoError(t, write.current())
	select {
	case <-write.done():
		t.Fatal("unrelated source/scope was canceled")
	default:
	}
	value := write.build()
	require.Equal(t, []byte{24, 10, 0, 0}, value.native)
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	// The recording connection has no connected endpoint for next-hop self.
	// This independent local rail uses an explicit valid forwarding hop.
	batch := bgptypes.NLRIBatch{Family: value.family,
		NLRIs:   []nlri.NLRI{nlri.NewINET(value.family, prefix, 0, false)},
		NextHop: bgptypes.NewNextHopExplicit(peer.Settings().LocalAddress)}
	require.NoError(t, writer.api.AnnounceNLRIBatch(t.Context(), selector.Addr(peer.Settings().Address), batch, producer.sender()))
	bodies := updateBodies(t, conn.written())
	require.Len(t, bodies, 1, "the unrelated authorized scope still writes a complete UPDATE")
	require.Equal(t, []netip.Prefix{prefix}, updatePrefixes(t, bodies[0], false))
	require.NoError(t, write.current())
}

func originRevocationAssertScope(t *testing.T, store *originReceiptStore, peer *Peer, expected uint64) {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	var current, receipts int
	for _, record := range store.current {
		if record.peer == peer {
			current++
			assert.Equal(t, expected, record.id)
		}
	}
	for _, record := range store.receipts {
		if record.peer == peer {
			receipts++
			assert.Equal(t, expected, record.id)
		}
	}
	want := 0
	if expected != 0 {
		want = 1
	}
	assert.Equal(t, want, current, "completed removal must not regain current state")
	assert.Equal(t, want, receipts, "completed removal must not regain a receipt")
}

// A paused real grant preflight cannot commit after removal ACK, including
// after regrant; matching physical Peer removal has the same ordering rule.
// A canceled join must keep the real write obligation, while another original
// Peer/source continues to write. All synchronization uses concrete wait entry.
func TestOriginIssuerRevocationFencesPreflightedIssue(t *testing.T) {
	for _, test := range []struct {
		name                string
		removePeer          bool
		regrantBeforeResume bool
	}{
		{name: "grant-removed"},
		{name: "grant-restored-before-resume", regrantBeforeResume: true},
		{name: "peer-removed", removePeer: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			work, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)
			writer, store, _, source, value := originReceiptFixture(t)
			t.Cleanup(func() {
				cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				require.NoError(t, store.close(cleanup))
			})
			pause := &originIssuePauseContext{Context: work, entered: make(chan struct{}), resume: make(chan struct{})}
			t.Cleanup(pause.release)
			// Capture this test process once; the fixture owner is already bound.
			owner, err := plugin.NewSelectedOwner(plugin.ProcessSender(selectedWriterProcess), work.Done(), nil)
			require.NoError(t, err)
			owner, err = owner.WithConnection(nilOriginCurrentReader, pause)
			require.NoError(t, err)
			producer, err := newProcessOriginProducer(owner)
			require.NoError(t, err)
			originalBus := writer.peer.settings.Bus.Clone()
			originalSnapshot := writer.peer.selectedOwnerSnapshot()
			id, err := store.issue(work, producer, source, value, writer.peer, owner)
			require.NoError(t, err)
			blocked, err := store.admit(id, owner, writer.peer, originalSnapshot, value.key)
			require.NoError(t, err)
			t.Cleanup(blocked.complete)
			healthyPeer, healthyConn := originRevocationScope(t, writer, "192.0.2.81", 72)
			healthySource, err := newOriginSource(originSourceInput{kind: originSourceLocalBGP, instance: 1})
			require.NoError(t, err)
			healthyID, err := store.issue(work, producer, healthySource, value, healthyPeer, owner)
			require.NoError(t, err)
			healthy, err := store.admit(healthyID, owner, healthyPeer, healthyPeer.selectedOwnerSnapshot(), value.key)
			require.NoError(t, err)
			t.Cleanup(healthy.complete)

			type issueResult struct {
				id  uint64
				err error
			}
			result := make(chan issueResult, 1)
			issueDone := make(chan struct{})
			pause.calls.Store(0)
			pause.armed.Store(true)
			go func() {
				defer close(issueDone)
				issued, err := store.issue(work, producer, source, value, writer.peer, owner)
				result <- issueResult{id: issued, err: err}
			}()
			t.Cleanup(func() { pause.release(); <-issueDone })
			select {
			case <-pause.entered:
			case <-work.Done():
				t.Fatal("issuer did not reach the post-preflight pause")
			}
			if test.removePeer {
				writer.peer.stopping.Store(true)
			} else {
				removed := originalBus.Clone()
				removed.Receive = plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true}
				writer.peer.writeBus(writer.api.r, removed)
				originRevocationPublishGraph(t, writer)
			}
			require.Same(t, originalSnapshot, writer.peer.selectedOwnerSnapshot(), "the preflight publication itself did not change")
			revoke := func(ctx context.Context) error {
				if test.removePeer {
					return store.revokePeer(ctx, writer.peer)
				}
				return store.revokeProducerPeer(ctx, producer, writer.peer)
			}
			canceled, stop := context.WithCancel(work)
			stop()
			require.ErrorIs(t, revoke(canceled), context.Canceled)
			require.Error(t, blocked.current())
			select {
			case <-blocked.done():
			default:
				t.Fatal("revocation did not cancel the admitted writer before join")
			}
			store.mu.Lock()
			retained := store.receipts[id]
			store.mu.Unlock()
			require.NotNil(t, retained, "a canceled join must retain its real writer obligation")
			joining := &originRevocationWaitContext{Context: work, entered: make(chan struct{})}
			joined := make(chan error, 1)
			revokeDone := make(chan struct{})
			go func() { defer close(revokeDone); joined <- revoke(joining) }()
			t.Cleanup(func() { blocked.complete(); <-revokeDone })
			select {
			case <-joining.entered:
			case err := <-joined:
				t.Fatalf("removal acknowledged before the retained writer completed: %v", err)
			case <-work.Done():
				t.Fatal("revoker did not join the retained writer")
			}
			originRevocationHealthyOutput(t, writer, healthyPeer, healthyConn, healthy, producer)
			copied := blocked
			copied.complete()
			blocked.complete()
			select {
			case err := <-joined:
				require.NoError(t, err)
			case <-work.Done():
				t.Fatal("removal did not acknowledge after actual writer completion")
			}

			var fresh uint64
			if test.regrantBeforeResume {
				writer.peer.writeBus(writer.api.r, originalBus)
				originRevocationPublishGraph(t, writer)
				fresh, err = store.issue(work, producer, source, value, writer.peer, owner)
				require.NoError(t, err)
			}
			before := store.next
			pause.release()
			select {
			case out := <-result:
				assert.Error(t, out.err, "a preflighted predecessor cannot commit after removal ACK")
				assert.Zero(t, out.id)
			case <-work.Done():
				t.Fatal("revoked issuer did not finish")
			}
			assert.Equal(t, before, store.next, "refused intake must issue no new identity")
			originRevocationAssertScope(t, store, writer.peer, fresh)
			require.NoError(t, healthy.current(), "the unrelated current receipt remains useful after ACK")
			if test.removePeer {
				replacement, _ := originRevocationScope(t, writer, writer.peer.Settings().Address.String(), 73)
				fresh, err = store.issue(work, producer, source, value, replacement, owner)
				require.NoError(t, err)
				write, err := store.admit(fresh, owner, replacement, replacement.selectedOwnerSnapshot(), value.key)
				require.NoError(t, err)
				require.NoError(t, write.current(), "a new same-address original Peer/Session remains admissible")
				write.complete()
				originRevocationAssertScope(t, store, writer.peer, 0)
			} else {
				writer.peer.writeBus(writer.api.r, originalBus)
				originRevocationPublishGraph(t, writer)
				if fresh == 0 {
					fresh, err = store.issue(work, producer, source, value, writer.peer, owner)
					require.NoError(t, err)
				}
				write, err := store.admit(fresh, owner, writer.peer, writer.peer.selectedOwnerSnapshot(), value.key)
				require.NoError(t, err)
				require.NoError(t, write.current(), "fresh regrant is not permanently sealed by old history")
				write.complete()
			}
			healthy.complete()
		})
	}
}
