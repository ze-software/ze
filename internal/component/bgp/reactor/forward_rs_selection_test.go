// Design: docs/architecture/bgp/structural-forwarding.md -- per-section destination fan-out.
// Related: forward_rs.go -- reactorForwardRSSection destination selection.
package reactor

import (
	"bufio"
	"bytes"
	"io"
	"net/netip"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// BenchmarkReactorForwardRSMixedRetry measures the actual mixed-field retry at
// increasing fan-out, including allocation costs. Destinations receive a legacy
// announcement and an IPv6 withdrawal through synchronous session writers, so
// worker scheduling and an ever-growing capture buffer cannot mask selection.
func BenchmarkReactorForwardRSMixedRetry(b *testing.B) {
	fwdLogger()
	level := slogutil.ListLevels()["bgp.reactor.forward"]
	if err := slogutil.SetLevel("bgp.reactor.forward", "disabled"); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := slogutil.SetLevel("bgp.reactor.forward", level); err != nil {
			b.Error(err)
		}
	})
	for _, n := range []int{100, 1000, 2000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			h := newFanoutHarnessWith(b, n, 1, fanoutOpts{groups: true})
			r := h.adapter.r
			source := r.peers[netip.AddrPortFrom(h.update.SourcePeerIP, DefaultBGPPort)]
			if source == nil {
				b.Fatal("source fixture missing")
			}
			body, err := message.UnpackUpdate(fanoutPayload())
			if err != nil {
				b.Fatal(err)
			}
			mp := []byte{0, 2, 1, 16}
			mp = append(mp, netip.MustParseAddr("fe80::1").AsSlice()...)
			mp = append(mp, 0, 64, 0x20, 0x01, 0x0d, 0xb8, 0, 7, 0, 0)
			body.PathAttributes = append(body.PathAttributes, makeAttr(0x80, 14, mp)...)
			wire := wireu.NewWireUpdate(buildModTestPayload(body.PathAttributes, body.NLRI), h.update.WireUpdate.SourceCtxID())
			wire.SetMessageID(1)
			h.update.WireUpdate = wire
			h.update.receivedPeer = source
			h.update.receivedGeneration = source.forwardGeneration.Load()
			for _, peer := range h.dests {
				peer.settings.RSClient = true
				peer.settings.PeerAS = 65002
				peer.negotiated.Load().families[family.IPv6Unicast] = true
				peer.refreshForwardFacts()
				peer.llScope.Store(newLinkScopeFrom(nil, peer.Settings().Address))
				rsSelectionWriter(b, peer, io.Discard)
			}
			// Prove the fixture actually selects every destination for retry;
			// an ordinary fan-out would hide the quadratic membership lookup.
			_, dispatched, selected := reactorForwardRSSection(r, h.update, wire, nil, 1, h.update.SourcePeerIP, source)
			if dispatched != 0 || len(selected) != n {
				b.Fatalf("initial mixed pass dispatched %d, selected %d; want 0, %d", dispatched, len(selected), n)
			}
			run := func() {
				skipped, dispatched := reactorForwardRS(r, h.update, 1, h.update.SourcePeerIP, source)
				if len(skipped) != 0 || dispatched != n {
					b.Fatalf("mixed retry skipped %d, dispatched %d; want 0, %d", len(skipped), dispatched, n)
				}
			}
			run()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				run()
			}
			b.StopTimer()
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/(float64(b.N)*float64(n)), "ns/dest")
		})
	}
}

// rsSelectionWriter gives a destination the same synchronous write path used by
// the RS rail in production. The caller chooses capture or allocation-free sink.
func rsSelectionWriter(t testing.TB, peer *Peer, writer io.Writer) *Session {
	t.Helper()
	session := NewSession(peer.Settings())
	session.adjOut = &peer.adjOut
	session.SetSourceID(peer.SourceID())
	session.setSendCtxID(peer.sendContextID())
	for _, event := range []fsm.Event{fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg} {
		if err := session.fsm.Event(event); err != nil {
			t.Fatal(err)
		}
	}
	session.bufWriter = bufio.NewWriterSize(writer, message.MaxMsgLen)
	peer.session = session
	t.Cleanup(session.timers.StopAll)
	return session
}

// TestReactorForwardRSRetrySelection keeps an excluded destination, removes one,
// replaces another at the same key, tears down one, and activates policy on one
// between passes. Only the selected, current, unfiltered peer receives bytes.
// An IBGP RR-client source is outside the selected slice: its classification
// must still allow reflection to the non-client and supply ORIGINATOR_ID.
func TestReactorForwardRSRetrySelection(t *testing.T) {
	h := newFanoutHarnessWith(t, 7, 1, fanoutOpts{groups: true})
	r := h.adapter.r
	source := r.peers[netip.AddrPortFrom(h.update.SourcePeerIP, DefaultBGPPort)]
	if source == nil {
		t.Fatal("source fixture missing")
	}
	h.update.receivedPeer = source
	h.update.receivedGeneration = source.forwardGeneration.Load()
	source.settings.PeerAS = source.settings.LocalAS
	source.settings.RouteReflectorClient = true
	source.remoteRouterID.Store(0x0a000001)
	source.refreshForwardFacts()
	var output [7]bytes.Buffer
	var sessions [7]*Session
	for i, peer := range h.dests {
		sessions[i] = rsSelectionWriter(t, peer, &output[i])
	}
	selected := append([]*Peer(nil), h.dests[:5]...)
	delete(r.peers, h.dests[1].Settings().PeerKey())
	replacement := NewPeer(h.dests[2].Settings())
	replacement.state.Store(int32(PeerStateEstablished))
	replacement.negotiated.Store(h.dests[2].negotiated.Load())
	replacement.sendCtx.Store(h.dests[2].sendCtx.Load())
	replacement.sendCtxID = h.dests[2].sendCtxID
	replacement.refreshForwardFacts()
	var replacementOutput bytes.Buffer
	replacementSession := rsSelectionWriter(t, replacement, &replacementOutput)
	r.peers[replacement.Settings().PeerKey()] = replacement
	h.dests[3].fwdFacts.Store(nil)
	for _, i := range []int{4, 6} {
		h.dests[i].settings.ExportFilters = frefs("bgp-rs:active")
		h.dests[i].refreshForwardFacts()
	}
	skipped, dispatched, retry := reactorForwardRSSection(r, h.update, h.update.WireUpdate, selected, 1, h.update.SourcePeerIP, source)
	if dispatched != 1 || len(retry) != 0 {
		t.Fatalf("dispatched %d, retry %d; want 1, 0", dispatched, len(retry))
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped %v; want both active-filter destinations", skipped)
	}
	for _, key := range skipped {
		if key != h.dests[4].Settings().PeerKey() && key != h.dests[6].Settings().PeerKey() {
			t.Fatalf("unexpected skipped peer %v", key)
		}
	}
	for i, session := range sessions {
		if err := session.bufWriter.Flush(); err != nil {
			t.Fatal(err)
		}
		if i != 0 && output[i].Len() != 0 {
			t.Fatalf("ineligible destination %d received %x", i, output[i].Bytes())
		}
	}
	if err := replacementSession.bufWriter.Flush(); err != nil {
		t.Fatal(err)
	}
	if replacementOutput.Len() != 0 {
		t.Fatalf("replacement received %x from stale selection", replacementOutput.Bytes())
	}
	if output[0].Len() <= message.HeaderLen {
		t.Fatal("eligible reflected destination received no UPDATE")
	}
	body, err := message.UnpackUpdate(output[0].Bytes()[message.HeaderLen:])
	if err != nil {
		t.Fatal(err)
	}
	_, _, originator, found := attribute.AttrFind(body.PathAttributes, attribute.AttrOriginatorID)
	if !found || !bytes.Equal(originator, []byte{10, 0, 0, 1}) {
		t.Fatalf("reflected ORIGINATOR_ID %x, present %v", originator, found)
	}
	if !bytes.Equal(body.NLRI, []byte{24, 10, 0, 0, 24, 10, 0, 1}) {
		t.Fatalf("reflected NLRI %x", body.NLRI)
	}
}
