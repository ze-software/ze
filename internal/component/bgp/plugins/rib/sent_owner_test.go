// Design: docs/architecture/plugin/rib-storage-design.md -- captured sent receipts.
package rib

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSentCleanupKeepsReceiptAfterProjectionRemoval exercises the actual RIB
// delete-before-dispatch boundary. A replacement arriving in that interval must
// not change the expected owner carried by the queued cleanup command.
func TestSentCleanupKeepsReceiptAfterProjectionRemoval(t *testing.T) {
	r := newTestRIBManager(t)
	source := netip.MustParseAddr("192.0.2.10")
	destination := netip.MustParseAddr("192.0.2.20")
	raw := []byte{24, 192, 0, 2}
	event := nativeSentEvent(t, destination, family.IPv4Unicast, raw, nativeSentAttrs(), false, false)
	event.MsgID = 18446744073709551614
	r.handleSent(event)
	r.peerMu.Lock()
	writes := r.pruneSentSourceLocked(source, nil)
	r.peerMu.Unlock()
	if len(writes) != 1 || len(r.ribOut[destination]) != 0 {
		t.Fatalf("cleanup did not remove exactly one projected route: writes=%d", len(writes))
	}
	replacement := nativeSentEvent(t, destination, family.IPv4Unicast, raw, nativeSentAttrs(), false, false)
	replacement.MsgID = 18446744073709551615
	replacement.RouteMeta["source-peer"] = "192.0.2.11"
	r.handleSent(replacement)
	t.Cleanup(func() {
		for _, routes := range r.ribOut[destination] {
			for _, entry := range routes {
				entry.release()
			}
		}
	})
	calls := 0
	r.updateHook = func(_ string, meta map[string]any) {
		calls++
		if meta[bgptypes.SentOwnerMessageMeta] != "18446744073709551614" {
			t.Errorf("cleanup lost old captured receipt: %v", meta)
		}
	}
	r.dispatchSentLifecycle(writes)
	if calls != 1 {
		t.Fatalf("cleanup dispatch count = %d", calls)
	}
}

// TestSentReplayGroupsKeepDistinctReceipts ensures attribute deduplication never
// merges routes whose captured advertisement revisions differ.
func TestSentReplayGroupsKeepDistinctReceipts(t *testing.T) {
	r := newTestRIBManager(t)
	destination := netip.MustParseAddr("192.0.2.20")
	for i, raw := range [][]byte{{24, 192, 0, 2}, {24, 198, 51, 100}} {
		event := nativeSentEvent(t, destination, family.IPv4Unicast, raw, nativeSentAttrs(), false, false)
		event.MsgID = uint64(i + 101)
		r.handleSent(event)
	}
	t.Cleanup(func() {
		for _, routes := range r.ribOut[destination] {
			for _, entry := range routes {
				entry.release()
			}
		}
	})
	r.peerMu.RLock()
	groups := r.collectGroupedRibOutRoutes(destination)
	r.peerMu.RUnlock()
	if len(groups) != 2 {
		t.Fatalf("replay merged distinct sent owners into %d groups", len(groups))
	}
	for _, group := range groups {
		if len(group.Prefixes) != 1 {
			t.Fatal("replay group contains multiple revisions")
		}
		if group.MinMsgID != 101 && group.MinMsgID != 102 {
			t.Fatalf("replay receipt = %s", strconv.FormatUint(group.MinMsgID, 10))
		}
	}
}

// TestSentSourceProvenanceExternalJSON uses the production full-event encoder
// and external RIB parser, not a hand-built metadata map at the receiving end.
func TestSentSourceProvenanceExternalJSON(t *testing.T) {
	r := newTestRIBManager(t)
	destination := netip.MustParseAddr("192.0.2.20")
	fam := family.IPv4Unicast
	attrs := nativeSentAttrs()
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	body = append(body, 24, 192, 0, 2, 24, 198, 51, 100)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, nil))
	if err != nil {
		t.Fatal(err)
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	wireAttrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, RawBytes: body,
		MessageID: 91, WireUpdate: wu, AttrsWire: wireAttrs, Direction: rpc.DirectionSent,
		SourcePeerStr: "192.0.2.10", SourceID: 17, SourceMessageID: 81,
		SourceOwner: 18446744073709551615,
		SentPathSources: []wireu.SentPathSource{
			{Family: fam, Ordinal: 0, PathID: 0},
			{Family: fam, Ordinal: 1, PathID: 4294967295},
		}}
	encoded := format.AppendSentMessage(nil, &plugin.PeerInfo{Address: destination}, msg,
		bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull})
	event, err := parseEvent(encoded)
	if err != nil {
		t.Fatalf("parse external event: %v", err)
	}
	r.handleSent(event)
	t.Cleanup(func() {
		for _, entry := range r.ribOut[destination][fam] {
			entry.release()
		}
	})
	for i, prefix := range []string{"192.0.2.0/24", "198.51.100.0/24"} {
		entry, exists := r.ribOut[destination][fam][ribOutKey{Prefix: netip.MustParsePrefix(prefix)}]
		if !exists || entry.SourceOwner != 18446744073709551615 || entry.SourceID != 17 ||
			entry.SourceMessageID != 81 || !entry.SourceAddPath || entry.LocalOrigin ||
			entry.SourcePeer != "192.0.2.10" {
			t.Fatalf("external source identity lost for %s: %+v", prefix, entry)
		}
		want := uint32(0)
		if i != 0 {
			want = 4294967295
		}
		if entry.SourcePath != want {
			t.Fatalf("external source path %s = %d, want %d", prefix, entry.SourcePath, want)
		}
	}
}

func TestInitialReplayReceiptExternalStateJSON(t *testing.T) {
	encoded := format.AppendStateChange(nil, &plugin.PeerInfo{
		Address: netip.MustParseAddr("192.0.2.20"), InitialReplay: 18446744073709551615,
	}, rpc.SessionStateUp, "", nil, plugin.EncodingJSON)
	event, err := parseEvent(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if event.InitialReplay != 18446744073709551615 {
		t.Fatalf("peer-up receipt rounded across external JSON: %d", event.InitialReplay)
	}
}

func TestInitialReplayRequiresExactReceivedHistory(t *testing.T) {
	for _, change := range []string{"unchanged", "new-message", "other-path", "removed", "lost-origin"} {
		t.Run(change, func(t *testing.T) {
			r := newTestRIBManager(t)
			source := netip.MustParseAddr("192.0.2.10")
			destination := netip.MustParseAddr("192.0.2.20")
			fam := family.IPv4Unicast
			received := storage.NewPeerRIB(source.String())
			r.bgpPeers[source] = received
			t.Cleanup(received.Release)
			received.SetAddPath(fam, true)
			raw := []byte{0, 0, 0, 7, 24, 192, 0, 2}
			received.Insert(fam, nativeSentAttrs(), raw)
			received.ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) { entry.MsgID = 81 })
			event := nativeSentEvent(t, destination, fam, raw[4:], nativeSentAttrs(), false, false)
			event.MsgID = 91
			event.RouteMeta["source-message-id"] = float64(81)
			event.RouteMeta["source-id"] = float64(17)
			event.RouteMeta[bgptypes.SourceOwnerMeta] = "101"
			event.RouteMeta[bgptypes.SentPathSourcesMeta] = []wireu.SentPathSource{{Family: fam, PathID: 7}}
			if change == "lost-origin" {
				delete(event.RouteMeta, bgptypes.SourceOwnerMeta)
			}
			r.handleSent(event)
			t.Cleanup(func() {
				for _, entry := range r.ribOut[destination][fam] {
					entry.release()
				}
			})
			switch change {
			case "new-message":
				received.ModifyFamilyEntry(fam, raw, func(entry *storage.RouteEntry) { entry.MsgID = 82 })
			case "other-path":
				received.Remove(fam, raw)
				other := []byte{0, 0, 0, 8, 24, 192, 0, 2}
				received.Insert(fam, nativeSentAttrs(), other)
				received.ModifyFamilyEntry(fam, other, func(entry *storage.RouteEntry) { entry.MsgID = 81 })
			case "removed":
				received.Remove(fam, raw)
			}
			r.peerMu.Lock()
			groups := r.collectPeerUpReplay(destination, true)
			r.peerMu.Unlock()
			if change != "unchanged" {
				if len(groups) != 0 {
					t.Fatal("stale sent history became an initial replay")
				}
				return
			}
			if len(groups) != 1 || groups[0].SourcePath != 7 || !groups[0].SourceAddPath ||
				groups[0].SourceOwner != 101 || groups[0].LocalOrigin || groups[0].MinMsgID != 91 {
				t.Fatalf("exact forwarded history lost its owner: %+v", groups)
			}
		})
	}
}
