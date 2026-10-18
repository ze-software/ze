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
