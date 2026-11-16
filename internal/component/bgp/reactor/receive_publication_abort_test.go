// Design: docs/architecture/memory/lifetime-contracts.md -- received cache ownership
// Design: docs/architecture/behavior/peer-lifecycle.md -- failed receive publication
// Related: peer_run.go, reactor_notify.go -- aborts before and during publication
package reactor

import (
	"bytes"
	"context"
	"net/netip"
	"testing"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

// abortOwnershipReceiver fails after the test has inspected the actual adopted
// receipt. Retaining in inspect models a consumer that started before a later
// recipient failed; that borrow must outlive lookup admission's cancellation.
type abortOwnershipReceiver struct {
	MessageReceiver
	inspect     func(uint64)
	beforePanic func()
}

func (receiver abortOwnershipReceiver) OnMessageBatchReceived(_ *plugin.PeerInfo, messages []bgptypes.RawMessage) []int {
	receiver.inspect(messages[0].MessageID)
	if receiver.beforePanic != nil {
		receiver.beforePanic()
	}
	panic("test: received publication failed after ownership inspection")
}

// TestReceivePublicationAbortReleasesReadOwnership exercises the actual worker
// and read-task failure boundaries. The socket's UPDATE uses a real standard
// pool slot; joining the daemon fixture must return it, including an UPDATE
// adopted before a pre-handoff panic which never reached the worker.
func TestReceivePublicationAbortReleasesReadOwnership(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()
	for _, mode := range []string{"false", "true"} {
		for _, path := range []string{"nil receiver", "receiver panic", "queued receiver panic", "pre-handoff panic"} {
			_, baseline := combinedBufMuxStats()
			var cache *RecentUpdateCache
			t.Run("coalesce="+mode+"/"+path, func(t *testing.T) {
				t.Cleanup(env.ResetCache)
				t.Setenv("ze.bgp.reactor.coalesce", mode)
				env.ResetCache()
				v := lowLiveRouter(t, acceptingLiveSettings(t, "10.0.0.1", 65000, 65002))[0]
				p := v.peer
				cache = p.reactor.recentUpdates
				lowEventually(t, func() bool { return cache.Len() == 0 }, "initial receive cache drained")
				inspect := func(id uint64) {
					u, ok := cache.Get(id)
					if !ok || u.poolBuf.Buf == nil || u.poolBuf.ID == noPoolBufID || len(u.poolBuf.Buf) != 4096 {
						panic("test: failure boundary did not own a real standard read slot")
					}
				}
				entered := make(chan struct{})
				release := make(chan struct{})
				open := func() {
					select {
					case <-release:
					default:
						close(release)
					}
				}
				t.Cleanup(open)
				hooks := sessionEndHooks{}
				if path == "pre-handoff panic" {
					hooks.beforeHandOff = func() {
						ids := cache.List()
						inspect(ids[len(ids)-1])
						panic("test: read publication failed before handoff")
					}
				} else {
					hooks.deliveryReceiver = func(prior MessageReceiver) MessageReceiver {
						if path == "nil receiver" {
							ids := cache.List()
							inspect(ids[len(ids)-1])
							return nil
						}
						receiver := abortOwnershipReceiver{MessageReceiver: prior, inspect: inspect}
						if path == "queued receiver panic" {
							receiver.beforePanic = func() {
								close(entered)
								<-release
							}
						}
						return receiver
					}
				}
				p.sessionEndHooks.Store(&hooks)
				t.Cleanup(func() { p.sessionEndHooks.Store(nil) })
				publication := p.receivePublication.Load()
				ownershipLiveSend(t, v, nil, firstASAttrs(4, 65002), netip.MustParsePrefix("10.20.0.0/24"))
				if path == "queued receiver panic" {
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						t.Fatal("first real receipt did not enter the held receiver")
					}
					for _, prefix := range []string{"10.20.1.0/24", "10.20.2.0/24"} {
						ownershipLiveSend(t, v, nil, firstASAttrs(4, 65002), netip.MustParsePrefix(prefix))
					}
					lowEventually(t, func() bool { return cache.Len() == 3 }, "two further real receipts queued behind held publication")
					open()
				}
				abortOwnershipJoin(t, p, publication)
			})
			if cache != nil && cache.Len() != 0 {
				t.Errorf("%s: joined publication left %d adopted cache entries", path, cache.Len())
			}
			_, used := combinedBufMuxStats()
			if used != baseline {
				t.Errorf("%s: joined fixture holds %d pool bytes, started with %d", path, used, baseline)
			}
		}
	}
}

// TestReceivePublicationAbortProtectsRetainedReadOwnership prevents replacing
// the leak with an unconditional delete/return: a consumer already holds both
// the receipt and an adopted forward variant when publication fails. New
// lookup/retain admission must close, but both real slots remain owned and
// byte-identical until that consumer releases its original retain.
func TestReceivePublicationAbortProtectsRetainedReadOwnership(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()
	for _, mode := range []string{"false", "true"} {
		for _, path := range []string{"partial receiver", "pre-handoff panic"} {
			_, baseline := combinedBufMuxStats()
			t.Run("coalesce="+mode+"/"+path, func(t *testing.T) {
				t.Cleanup(env.ResetCache)
				t.Setenv("ze.bgp.reactor.coalesce", mode)
				env.ResetCache()
				v := lowLiveRouter(t, acceptingLiveSettings(t, "10.0.0.1", 65000, 65002))[0]
				p := v.peer
				cache := p.reactor.recentUpdates
				lowEventually(t, func() bool { return cache.Len() == 0 }, "initial receive cache drained")
				var id uint64
				var adopted *ReceivedUpdate
				var receipt, variant []byte
				inspect := func(messageID uint64) {
					id = messageID
					var ok bool
					adopted, ok = cache.Get(id)
					if !ok || adopted.poolBuf.ID == noPoolBufID || !cache.Retain(id) {
						panic("test: cannot retain the actual received pool buffer")
					}
					receipt = bytes.Clone(adopted.poolBuf.Buf)
					h := getReadBuf(false)
					if h.Buf == nil {
						panic("test: cannot borrow the forward variant")
					}
					for i := range h.Buf {
						h.Buf[i] = 0xa5
					}
					variant = h.Buf
					adopted.adoptFwdHandle(h)
				}
				hooks := sessionEndHooks{}
				if path == "pre-handoff panic" {
					hooks.beforeHandOff = func() {
						ids := cache.List()
						inspect(ids[len(ids)-1])
						panic("test: retained read failed before handoff")
					}
				} else {
					hooks.deliveryReceiver = func(prior MessageReceiver) MessageReceiver {
						return abortOwnershipReceiver{MessageReceiver: prior, inspect: inspect}
					}
				}
				p.sessionEndHooks.Store(&hooks)
				t.Cleanup(func() {
					p.sessionEndHooks.Store(nil)
					if id != 0 {
						cache.Release(id)
					}
				})
				publication := p.receivePublication.Load()
				ownershipLiveSend(t, v, nil, firstASAttrs(4, 65002), netip.MustParsePrefix("10.20.0.0/24"))
				abortOwnershipJoin(t, p, publication)
				if _, found := cache.Get(id); found {
					t.Error("failed publication still admits cache lookup")
				}
				if cache.Retain(id) {
					cache.Release(id)
					t.Error("failed publication still admits a new retain")
				}
				_, held := combinedBufMuxStats()
				if held != baseline+2*4096 {
					t.Errorf("retained receipt and variant hold %d bytes, want %d", held, baseline+2*4096)
				}
				if !bytes.Equal(adopted.poolBuf.Buf, receipt) || !bytes.Equal(variant, bytes.Repeat([]byte{0xa5}, 4096)) {
					t.Error("publication abort returned or changed a still-retained buffer")
				}
				cache.Release(id)
				id = 0
				if cache.Len() != 0 {
					t.Errorf("last retained consumer left %d cache entries", cache.Len())
				}
			})
			_, used := combinedBufMuxStats()
			if used != baseline {
				t.Errorf("%s: final release holds %d pool bytes, started with %d", path, used, baseline)
			}
		}
	}
}

func abortOwnershipJoin(t *testing.T, peer *Peer, publication *receivePublication) {
	t.Helper()
	select {
	case <-publication.done:
	case <-time.After(5 * time.Second):
		t.Fatal("failed publication worker did not finish")
	}
	peer.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := peer.Wait(ctx); err != nil {
		t.Fatalf("failed publication peer did not join: %v", err)
	}
	if !publication.failed.Load() {
		t.Fatal("receive publication did not record its failure")
	}
}
