// Design: docs/architecture/plugin/rib-storage-design.md -- sent projection of a recovery write.
package rib

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBSentProjectionOfPurgedOwnerRemoves proves R-12 at the RIB: the sent
// event of a recovery write whose survivor bgp-rib purged between the lookup
// and the projection removes the projected entry and queues exactly one
// removal for the dispatcher, at each of the three sent-store sites. Method:
// one received path from 192.0.2.10 (message 81), a sent event to 192.0.2.20
// naming that path as its candidate, and the received inventory changed
// before the projection. A newer revision, a present path, or a sent event
// without the recovery mark leaves the projection alone. Nothing is sent on
// the event path; the queued removal leaves through the dispatcher.
func TestRIBSentProjectionOfPurgedOwnerRemoves(t *testing.T) {
	sites := []string{"structured", "raw", "family-ops"}
	cases := []struct {
		name     string
		received string // gone, newer, present
		marked   bool
		removed  bool
	}{
		{name: "purged", received: "gone", marked: true, removed: true},
		{name: "newer-revision", received: "newer", marked: true},
		{name: "path-present", received: "present", marked: true},
		{name: "unmarked-purged", received: "gone"},
	}
	for _, site := range sites {
		for _, tc := range cases {
			t.Run(site+"/"+tc.name, func(t *testing.T) {
				r := newTestRIBManager(t)
				source := netip.MustParseAddr("192.0.2.10")
				destination := netip.MustParseAddr("192.0.2.20")
				fam := family.IPv4Unicast
				raw := []byte{24, 192, 0, 2}
				key := ribOutKey{Prefix: netip.MustParsePrefix("192.0.2.0/24")}
				received := storage.NewPeerRIB(source.String())
				r.bgpPeers[source] = received
				t.Cleanup(received.Release)
				received.Insert(fam, nativeSentAttrs(), raw)
				received.ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) { entry.MsgID = 81 })
				switch tc.received {
				case "gone":
					received.Remove(fam, raw)
				case "newer":
					received.ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) { entry.MsgID = 82 })
				}
				eventPath := 0
				r.updateHook = func(string, map[string]any) { eventPath++ }

				recoverySentEvent(t, r, site, destination, fam, raw, tc.marked)
				t.Cleanup(func() {
					for _, entry := range r.ribOut[destination][fam] {
						entry.release()
					}
				})

				r.peerMu.Lock()
				_, stored := r.ribOut[destination][fam][key]
				pending := append([]sentLifecycleWrite(nil), r.recoveryPending...)
				r.peerMu.Unlock()
				if eventPath != 0 {
					t.Fatalf("the projection sent %d writes on the event path", eventPath)
				}
				if !tc.removed {
					if !stored || len(pending) != 0 {
						t.Fatalf("projection changed: stored=%v pending=%d", stored, len(pending))
					}
					return
				}
				if stored {
					t.Fatal("the purged survivor's projected entry stayed in ribOut")
				}
				if len(pending) != 1 || pending[0].peer != destination.String() || pending[0].message != 91 {
					t.Fatalf("queued removals = %+v, want one to %s with receipt 91", pending, destination)
				}

				sent := make(chan sentLifecycleWrite, 2)
				r.startRecoveryRemovals(context.Background(), func(_ context.Context, write sentLifecycleWrite) error {
					sent <- write
					return nil
				})
				defer r.stopRecoveryRemovals()
				select {
				case write := <-sent:
					if write.command != pending[0].command {
						t.Fatalf("dispatcher sent %q, want %q", write.command, pending[0].command)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("the dispatcher never sent the queued removal")
				}
			})
		}
	}
}

// TestRecoveryRemovalDispatcherStopCancelsInFlight proves the dispatcher's
// stop: it cancels the send in flight, joins the worker, and drops the
// removals still pending. Method: two removals queued, a send that blocks
// until its context ends, then stop under a deadline.
func TestRecoveryRemovalDispatcherStopCancelsInFlight(t *testing.T) {
	r := newTestRIBManager(t)
	r.peerMu.Lock()
	r.queueRecoveryRemovalLocked(sentLifecycleWrite{peer: "192.0.2.20", command: "first", message: 1})
	r.peerMu.Unlock()
	started := make(chan struct{})
	calls := 0
	r.startRecoveryRemovals(context.Background(), func(ctx context.Context, _ sentLifecycleWrite) error {
		calls++
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the dispatcher never started the queued removal")
	}
	r.peerMu.Lock()
	r.queueRecoveryRemovalLocked(sentLifecycleWrite{peer: "192.0.2.20", command: "second", message: 2})
	r.peerMu.Unlock()

	stopped := make(chan struct{})
	go func() { // Bounded by the deadline below.
		r.stopRecoveryRemovals()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not cancel the send in flight and join the worker")
	}
	r.peerMu.Lock()
	pending := len(r.recoveryPending)
	removals := r.removals
	r.peerMu.Unlock()
	if calls != 1 || pending != 0 || removals != nil {
		t.Fatalf("after stop: sends=%d pending=%d dispatcher=%v, want 1, 0, nil", calls, pending, removals)
	}
}

// recoverySentEvent projects one sent UPDATE to destination at the named
// store site, carrying the candidate's provenance (source 192.0.2.10, owner
// 101, received message 81) and, when marked, the recovery-write meta.
func recoverySentEvent(t *testing.T, r *RIBManager, site string, destination netip.Addr, fam family.Family, raw []byte, marked bool) {
	t.Helper()
	if site == "structured" {
		ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{fam: false}))
		if err != nil {
			t.Fatal(err)
		}
		attrs := appendAttr(nativeSentAttrs(), byte(attribute.AttrNextHop), 0x40, []byte{192, 0, 2, 10})
		body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
		body = append(body, attrs...)
		body = append(body, raw...)
		wu := wireu.NewWireUpdate(body, ctxID)
		wireAttrs, err := wu.Attrs()
		if err != nil {
			t.Fatal(err)
		}
		meta := map[string]any{}
		if marked {
			meta[bgptypes.RecoveryWriteMeta] = true
		}
		r.handleSentStructured(&rpc.StructuredEvent{
			PeerAddress: destination.String(), SourcePeerStr: "192.0.2.10", MessageID: 91, Meta: meta,
			RawMessage: &bgptypes.RawMessage{WireUpdate: wu, AttrsWire: wireAttrs, SourceMessageID: 81, SourceOwner: 101},
		})
		return
	}
	event := nativeSentEvent(t, destination, fam, raw, nativeSentAttrs(), false, false)
	event.MsgID = 91
	event.RouteMeta["source-message-id"] = float64(81)
	event.RouteMeta[bgptypes.SourceOwnerMeta] = "101"
	if marked {
		event.RouteMeta[bgptypes.RecoveryWriteMeta] = true
	}
	if site == "family-ops" {
		event.RawNLRI = nil
		event.FamilyOps[fam][0].NLRIs = []any{"192.0.2.0/24"}
	}
	r.handleSent(event)
}

// TestRIBForwardedProjectionAfterLifecycleRemovalRemoves proves R-12 for a
// forwarded write. A lifecycle removal (release-routes or purge-stale) of a
// source's path runs while the sent event of a forwarded write of that path
// is still queued, so the removal finds no sent entry to withdraw; the late
// projection then removes the entry and queues the one removal the lifecycle
// missed, at each of the three sent-store sites. Method: received message 81
// from 192.0.2.10 for 192.0.2.0/24 through the real received handler, the
// lifecycle command, then an unmarked sent event to 192.0.2.20 naming message
// 81. The fence keeps every other absence alone: a received message not yet
// applied when the removal ran (the fast path forwards before the received
// event is queued), a received withdrawal with no lifecycle removal since
// (its own forward reaches the destination), and a newer revision.
//
// VALIDATES: a GR purge of a path whose forwarded copy was not yet projected
// still withdraws it from the destination.
// PREVENTS: a client keeping a route its source no longer has, with nothing
// left to remove it (TestOwedKeyGRPurgeRecoversSurvivor, about 1 run in 16).
func TestRIBForwardedProjectionAfterLifecycleRemovalRemoves(t *testing.T) {
	sites := []string{"structured", "raw", "family-ops"}
	cases := []struct {
		name    string
		removed bool
		before  func(t *testing.T, r *RIBManager, source netip.Addr)
	}{
		{name: "release", removed: true, before: func(t *testing.T, r *RIBManager, source netip.Addr) {
			t.Helper()
			lifecycleReceive(t, r, source, 81, false)
			r.retainRoutes(source.String(), nil, false)
			r.releaseRoutes(source.String())
		}},
		{name: "purge-stale", removed: true, before: func(t *testing.T, r *RIBManager, source netip.Addr) {
			t.Helper()
			lifecycleReceive(t, r, source, 81, false)
			lifecycleMarkAndPurge(t, r, source)
		}},
		{name: "not-applied-before-removal", before: func(t *testing.T, r *RIBManager, source netip.Addr) {
			t.Helper()
			lifecycleReceive(t, r, source, 70, true)
			r.retainRoutes(source.String(), nil, false)
			r.releaseRoutes(source.String())
		}},
		{name: "withdrawn-no-lifecycle", before: func(t *testing.T, r *RIBManager, source netip.Addr) {
			t.Helper()
			lifecycleReceive(t, r, source, 81, false)
			lifecycleWithdraw(t, r, source, 82)
		}},
		{name: "newer-revision-after-purge", before: func(t *testing.T, r *RIBManager, source netip.Addr) {
			t.Helper()
			lifecycleReceive(t, r, source, 81, false)
			lifecycleMarkAndPurge(t, r, source)
			lifecycleReceive(t, r, source, 83, false)
		}},
	}
	for _, site := range sites {
		for _, tc := range cases {
			t.Run(site+"/"+tc.name, func(t *testing.T) {
				r := newTestRIBManager(t)
				source := netip.MustParseAddr("192.0.2.10")
				destination := netip.MustParseAddr("192.0.2.20")
				fam := family.IPv4Unicast
				key := ribOutKey{Prefix: netip.MustParsePrefix("192.0.2.0/24")}
				t.Cleanup(func() {
					for _, received := range r.bgpPeers {
						received.Release()
					}
					for _, families := range r.ribOut {
						for _, routes := range families {
							for _, entry := range routes {
								entry.release()
							}
						}
					}
				})
				lifecycle := 0
				r.updateHook = func(string, map[string]any) { lifecycle++ }
				tc.before(t, r, source)
				if lifecycle != 0 {
					t.Fatalf("the lifecycle command sent %d removals with no projected copy to remove", lifecycle)
				}

				recoverySentEvent(t, r, site, destination, fam, []byte{24, 192, 0, 2}, false)

				r.peerMu.Lock()
				_, stored := r.ribOut[destination][fam][key]
				pending := append([]sentLifecycleWrite(nil), r.recoveryPending...)
				r.peerMu.Unlock()
				if lifecycle != 0 {
					t.Fatalf("the projection sent %d writes on the event path", lifecycle)
				}
				if !tc.removed {
					if !stored || len(pending) != 0 {
						t.Fatalf("projection changed: stored=%v pending=%d", stored, len(pending))
					}
					return
				}
				if stored {
					t.Fatal("the purged path's late projected entry stayed in ribOut")
				}
				if len(pending) != 1 || pending[0].peer != destination.String() || pending[0].message != 91 {
					t.Fatalf("queued removals = %+v, want one to %s with receipt 91", pending, destination)
				}
				want := "update text nlri del 192.0.2.0/24"
				if pending[0].command != want {
					t.Fatalf("queued removal %q, want %q", pending[0].command, want)
				}
			})
		}
	}
}

// lifecycleReceive applies one received UPDATE from source through the
// structured received handler as message: 192.0.2.0/24, or 198.51.100.0/24
// when other is set.
func lifecycleReceive(t *testing.T, r *RIBManager, source netip.Addr, message uint64, other bool) {
	t.Helper()
	nlri := []byte{24, 192, 0, 2}
	if other {
		nlri = []byte{24, 198, 51, 100}
	}
	attrs := appendAttr(nativeSentAttrs(), byte(attribute.AttrNextHop), 0x40, []byte{192, 0, 2, 10})
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	body = append(body, nlri...)
	lifecycleFeed(t, r, source, message, body)
}

// lifecycleWithdraw applies one received withdrawal of 192.0.2.0/24 from
// source as message.
func lifecycleWithdraw(t *testing.T, r *RIBManager, source netip.Addr, message uint64) {
	t.Helper()
	lifecycleFeed(t, r, source, message, []byte{0, 4, 24, 192, 0, 2, 0, 0})
}

// lifecycleFeed drives handleReceivedStructured with one IPv4 unicast UPDATE
// body carrying message as its received message ID.
func lifecycleFeed(t *testing.T, r *RIBManager, source netip.Addr, message uint64, body []byte) {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: false}))
	if err != nil {
		t.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetMessageID(message)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: source.String(), PeerAS: 65001, LocalAS: 65000,
		RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body, WireUpdate: wu,
			AttrsWire: attrs, MessageID: message},
	})
}

// lifecycleMarkAndPurge marks every route of source GR stale, then purges the
// stale routes, as an End-of-RIB after a reconnect does.
func lifecycleMarkAndPurge(t *testing.T, r *RIBManager, source netip.Addr) {
	t.Helper()
	if _, _, err := r.markStaleCommand([]string{source.String(), "0"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.purgeStaleCommand([]string{source.String()}); err != nil {
		t.Fatal(err)
	}
}
