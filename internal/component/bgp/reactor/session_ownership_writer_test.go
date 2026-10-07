// Design: docs/architecture/update-building.md -- final writer ownership and flush lifecycle
// Related: session_write.go -- real encoded/raw writers, not an ownership model
package reactor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func ownershipWriterPeer(t *testing.T) (*Peer, *recordingConn, *Peer) {
	t.Helper()
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	peer.session.adjOut = &peer.adjOut
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	peer.session.sendCtxID = ctxID
	source := makeForwardSourcePeer(t, ctx, ctxID)
	return peer, conn, source
}

// ownershipWriterUpdate borrows valid UPDATE sections for a synchronous send.
// RFC 4271 Section 4.3: "An UPDATE message is used to advertise feasible routes
// that share common path attributes to a peer, or to withdraw multiple
// unfeasible routes from service (see 3.1)."
func ownershipWriterUpdate(t *testing.T, body []byte) *message.Update {
	t.Helper()
	sec, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatal(err)
	}
	return &message.Update{WithdrawnRoutes: sec.Withdrawn(body), PathAttributes: sec.Attrs(body), NLRI: sec.NLRI(body)}
}

func ownershipWriterForward(t *testing.T, target, source *Peer, body []byte, encoded bool) error {
	t.Helper()
	s := target.currentSession()
	item := fwdItem{peer: target, session: s, receivedPeer: source,
		receivedGeneration: source.forwardGeneration.Load(), authority: adjOutForwarded}
	item.rawBodies = [][]byte{body}
	original := wireu.NewWireUpdate(body, source.recvCtxID)
	// RFC 7911 Section 2: retain source paths before the final writer.
	if err := prepareFwdProvenance(&item, original, original, false); err != nil {
		if item.provenance != nil {
			item.provenance.release()
		}
		return err
	}
	if item.provenance != nil {
		defer item.provenance.release()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.sentForward = &item
	defer func() { s.sentForward = nil }()
	var err error
	if encoded {
		// RFC 4271 Section 4.3.
		err = s.writeUpdatePreFiltered(ownershipWriterUpdate(t, body))
	} else {
		err = s.writeRawUpdateBody(body)
	}
	if err != nil {
		return err
	}
	return s.flushWrites()
}

// TestFinalWriterLocalOwnership proves actual local writes displace forwarded
// ownership on both writer rails, and a forward clears local duplicate evidence.
func TestFinalWriterLocalOwnership(t *testing.T) {
	for _, encoded := range []bool{false, true} {
		name := "raw"
		if encoded {
			name = "encoded"
		}
		t.Run(name, func(t *testing.T) {
			peer, conn, source := ownershipWriterPeer(t)
			// RFC 4271 Section 4.3.
			announce := ownershipWriterUpdate(t, syncOrderAnnounceBody)
			if err := ownershipWriterForward(t, peer, source, syncOrderAnnounceBody, encoded); err != nil {
				t.Fatal(err)
			}
			if err := peer.session.SendUpdate(announce); err != nil {
				t.Fatal(err)
			}
			if err := ownershipWriterForward(t, peer, source, syncOrderWithdrawBody, encoded); err != nil {
				t.Fatal(err)
			}
			if err := peer.session.SendUpdate(announce); err != nil {
				t.Fatal(err)
			}
			if got := parseWireUpdates(t, conn.written()); len(got) != 2 || !got[0].announces || !got[1].announces {
				t.Fatalf("local owner was withdrawn or duplicated: %+v", got)
			}
			if err := ownershipWriterForward(t, peer, source, syncOrderAnnounceBody, encoded); err != nil {
				t.Fatal(err)
			}
			if err := peer.session.SendUpdate(announce); err != nil {
				t.Fatal(err)
			}
			if got := parseWireUpdates(t, conn.written()); len(got) != 4 {
				t.Fatalf("forwarded replacement left stale local duplicate evidence: %+v", got)
			}
			if err := ownershipWriterForward(t, peer, source, syncOrderAnnounceBody, encoded); err != nil {
				t.Fatal(err)
			}
			// RFC 4271 Section 4.3: explicit local withdrawal remains authorized.
			if err := peer.session.SendUpdate(ownershipWriterUpdate(t, syncOrderWithdrawBody)); err != nil {
				t.Fatal(err)
			}
			got := parseWireUpdates(t, conn.written())
			if len(got) != 6 || !got[5].withdraws || got[5].endOfRIB {
				t.Fatalf("authorized local withdrawal missing: %+v", got)
			}
		})
	}
}

// TestFinalWriterLabelOnlyChange separates canonical route identity from exact
// local advertisement evidence by sending two labels and repeating the second.
func TestFinalWriterLabelOnlyChange(t *testing.T) {
	peer, conn, _ := ownershipWriterPeer(t)
	first := []byte{48, 0, 6, 0x41, 192, 0, 2}
	second := []byte{48, 0, 6, 0x51, 192, 0, 2}
	for _, raw := range [][]byte{first, second, second} {
		// RFC 4760 Section 3 and RFC 8277 Section 2.1.
		attrs := append([]byte{0x40, 1, 1, 0, 0x40, 2, 0}, buildMPReachSource(1, 4, []byte{10, 0, 0, 1}, raw)...)
		if err := peer.session.SendUpdate(&message.Update{PathAttributes: attrs}); err != nil {
			t.Fatal(err)
		}
	}
	bodies := updateBodies(t, conn.written())
	if len(bodies) != 2 {
		t.Fatalf("label-only change or duplicate repeat mishandled: %d UPDATEs", len(bodies))
	}
	if !bytes.Contains(bodies[0], first) || !bytes.Contains(bodies[1], second) {
		t.Fatalf("labels differ from the two final advertisements: %x", bodies)
	}
	if peer.adjOut.suppressedCount() != 1 {
		t.Fatalf("identical second-label repeat was not counted: %d", peer.adjOut.suppressedCount())
	}
}

// ownershipWriterFailure records the accepted prefix of a deliberately failed
// real bufio write. No success bytes are invented by the fixture.
type ownershipWriterFailure struct {
	recordingConn
	short  bool
	closed bool
}

func (c *ownershipWriterFailure) Write(data []byte) (int, error) {
	if c.short {
		return c.recordingConn.Write(data[:len(data)/2])
	}
	return 0, io.ErrClosedPipe
}

func (c *ownershipWriterFailure) Close() error {
	c.closed = true
	return nil
}

// TestFinalWriterFailureRetiresExactSession exercises failed direct writes and
// failed/short flushes, rejects later sends, then proves a replacement still works.
func TestFinalWriterFailureRetiresExactSession(t *testing.T) {
	for _, test := range []struct {
		name  string
		size  int
		short bool
	}{
		{name: "write-error", size: 16},
		{name: "flush-error", size: 4096},
		{name: "short-flush", size: 4096, short: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer, _, _ := ownershipWriterPeer(t)
			old := peer.session
			failed := &ownershipWriterFailure{short: test.short}
			old.conn = failed
			old.bufWriter = bufio.NewWriterSize(failed, test.size)
			// RFC 4271 Section 4.3.
			update := ownershipWriterUpdate(t, syncOrderAnnounceBody)
			if err := old.SendUpdate(update); err == nil {
				t.Fatal("failed connection reported successful send")
			}
			if !failed.closed || !old.tearingDown.Load() || old.writeFailed == nil {
				t.Fatal("failed session was not sealed and closed")
			}
			if len(peer.adjOut.routes) != 0 {
				t.Fatal("failed advertisement remained authoritative")
			}
			before := len(failed.written())
			if err := old.SendUpdate(update); err == nil {
				t.Fatal("failed session accepted another send")
			}
			if len(failed.written()) != before {
				t.Fatal("later send continued a failed stream")
			}
			replacement, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
			peer.session = replacement.session
			peer.session.adjOut = &peer.adjOut
			if err := peer.session.SendUpdate(update); err != nil {
				t.Fatal(err)
			}
			old.writeMu.Lock()
			err := old.flushWrites()
			old.writeMu.Unlock()
			if err == nil {
				t.Fatal("failed old flush lost its error")
			}
			if peer.session.tearingDown.Load() || len(updateBodies(t, conn.written())) != 1 {
				t.Fatal("old failure retired the replacement session")
			}
			if len(peer.adjOut.routes) != 1 {
				t.Fatal("old failure erased replacement ownership")
			}
		})
	}
}

// TestFinalWriterLateFlushFailureKeepsReplacement covers the first failure of
// an old buffered session after its peer has already installed a new writer.
func TestFinalWriterLateFlushFailureKeepsReplacement(t *testing.T) {
	peer, _, _ := ownershipWriterPeer(t)
	old := peer.session
	failed := &ownershipWriterFailure{}
	old.conn = failed
	old.bufWriter = bufio.NewWriterSize(failed, 4096)
	old.writeMu.Lock()
	err := old.writeRawUpdateBody(syncOrderAnnounceBody)
	old.writeMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	replacement, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	peer.session = replacement.session
	peer.session.adjOut = &peer.adjOut
	// RFC 4271 Section 4.3.
	if err := peer.session.SendUpdate(ownershipWriterUpdate(t, syncOrderAnnounceBody)); err != nil {
		t.Fatal(err)
	}
	old.writeMu.Lock()
	err = old.flushWrites()
	old.writeMu.Unlock()
	if !errors.Is(err, io.ErrClosedPipe) || !failed.closed {
		t.Fatalf("old pending flush did not fail closed: %v", err)
	}
	if peer.session.tearingDown.Load() || len(peer.adjOut.routes) != 1 || len(updateBodies(t, conn.written())) != 1 {
		t.Fatal("old flush failure changed replacement ownership or connection")
	}
}

// TestFinalWriterMixedWithdrawalIntent reaches the actual withdrawal builder
// and provenance producer: synthesized absent Q passes, original absent P does
// not. The later unframeable source withdrawal returns an error without output.
func TestFinalWriterMixedWithdrawalIntent(t *testing.T) {
	peer, conn, source := ownershipWriterPeer(t)
	s := peer.session
	q := []byte{24, 198, 51, 100}
	original := fwdPackUpdateBody(&message.Update{WithdrawnRoutes: syncOrderPrefixWire,
		PathAttributes: syncOrderAnnounceBody[4:24], NLRI: q})
	converted := make([]byte, message.MaxMsgLen)
	n := buildWithdrawalPayload(original, converted)
	if n == 0 {
		t.Fatal("real withdrawal synthesis produced no body")
	}
	converted = converted[:n]
	item := fwdItem{peer: peer, session: s, rawBodies: [][]byte{converted}, receivedPeer: source,
		receivedGeneration: source.forwardGeneration.Load(), authority: adjOutForwarded}
	originalWire := wireu.NewWireUpdate(original, s.sendCtxID)
	convertedWire := wireu.NewWireUpdate(converted, s.sendCtxID)
	if err := prepareFwdProvenance(&item, originalWire, convertedWire, true); err != nil {
		t.Fatal(err)
	}
	defer item.provenance.release()
	s.writeMu.Lock()
	s.sentForward = &item
	err := s.writeRawUpdateBody(converted)
	if err == nil {
		err = s.flushWrites()
	}
	s.sentForward = nil
	s.writeMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	bodies := updateBodies(t, conn.written())
	if len(bodies) != 1 {
		t.Fatalf("mixed synthesis wrote %d UPDATEs, want one", len(bodies))
	}
	sections, err := wire.ParseUpdateSections(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sections.Withdrawn(bodies[0]), q) || len(sections.NLRI(bodies[0])) != 0 {
		t.Fatalf("original withdrawal inherited synthesized authority: %x", bodies[0])
	}
	before := peer.sentUpdateSequence.Load()
	if err := ownershipWriterForward(t, peer, source, syncOrderWithdrawBody, false); err != nil {
		t.Fatal(err)
	}
	if len(updateBodies(t, conn.written())) != 1 {
		t.Fatal("entirely filtered withdrawal emitted an accidental EOR")
	}
	if peer.sentUpdateSequence.Load() == before {
		t.Fatal("filtered source operation left a cold recovery receipt current")
	}
	malformed := []byte{0, 2, 32, 192, 0, 0}
	if err := ownershipWriterForward(t, peer, source, malformed, false); err == nil {
		t.Fatal("unframeable source withdrawal passed wholesale")
	}
	if len(updateBodies(t, conn.written())) != 1 {
		t.Fatal("unframeable withdrawal reached the wire")
	}
	// The filtered malformed message did not corrupt the writer's frontier.
	if err := peer.session.sendWithdraw(netip.MustParsePrefix(syncOrderPrefix), false); err != nil {
		t.Fatal(err)
	}
	if len(updateBodies(t, conn.written())) != 2 {
		t.Fatal("explicit authorized local withdrawal stopped working")
	}
}

// TestFinalWriterInitialLocalReplay distinguishes deliberate peer-UP history
// from refresh work captured before reconnect, and from a newer live owner.
func TestFinalWriterInitialLocalReplay(t *testing.T) {
	for _, mode := range []string{"empty-local", "replay-fence-after-eor", "newer-forwarded", "expired-phase", "expired-policy", "old-session", "missing-origin"} {
		t.Run(mode, func(t *testing.T) {
			peer, _, source := ownershipWriterPeer(t)
			old := peer.session
			// RFC 4271 Section 4.3.
			update := ownershipWriterUpdate(t, syncOrderAnnounceBody)
			if err := old.SendUpdate(update); err != nil {
				t.Fatal(err)
			}
			receipt := old.sentReceipt.MessageID()
			replacement, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
			peer.session = replacement.session
			peer.session.adjOut = &peer.adjOut
			peer.initialSyncEOROwed.Store(true)
			target := announceTarget{peer: peer, session: peer.session}
			batch := bgptypes.NLRIBatch{Replay: true, SentOwnerMessage: receipt}
			if err := target.sendBatchUpdate(context.Background(), update, message.MaxMsgLen, false, batch); err != nil {
				t.Fatal(err)
			}
			if len(conn.written()) != 0 {
				t.Fatal("old same-session refresh populated the replacement")
			}
			batch.InitialReplay = peer.session.initialReplay
			batch.InitialLocal = true
			switch mode {
			case "replay-fence-after-eor":
				peer.initialSyncEOROwed.Store(false)
				peer.initialUpdateOwed.Store(true)
			case "newer-forwarded":
				if err := ownershipWriterForward(t, peer, source, syncOrderAnnounceBody, false); err != nil {
					t.Fatal(err)
				}
			case "expired-phase":
				peer.initialSyncEOROwed.Store(false)
			case "expired-policy":
				peer.initialSyncEOROwed.Store(false)
				peer.session.egressRouteFilter = func([]byte) (bool, []byte) { return true, nil }
			case "old-session":
				batch.InitialReplay = old.initialReplay
			case "missing-origin":
				batch.InitialLocal = false
			}
			err := target.sendBatchUpdate(context.Background(), update, message.MaxMsgLen, false, batch)
			switch mode {
			case "expired-phase", "expired-policy", "old-session", "missing-origin":
				if err == nil {
					t.Fatal("uncaptured replay authority was accepted")
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "empty-local", "replay-fence-after-eor":
				if got := parseWireUpdates(t, conn.written()); len(got) != 1 || !got[0].announces {
					t.Fatalf("explicit reconnect-local replay was not sent: %+v", got)
				}
				if peer.session.sentReceipt.MessageID() == receipt {
					t.Fatal("initial replay reused the old wire message ID")
				}
				// The old sent receipt is retained because replay feedback is
				// not a new RIB projection entry. Refresh must still send it.
				batch.InitialReplay = 0
				if err := target.sendBatchUpdate(context.Background(), update, message.MaxMsgLen, false, batch); err != nil {
					t.Fatal(err)
				}
				if got := parseWireUpdates(t, conn.written()); len(got) != 2 || !got[1].announces {
					t.Fatalf("initial replay lost its original sent receipt: %+v", got)
				}
			case "newer-forwarded":
				if got := parseWireUpdates(t, conn.written()); len(got) != 1 {
					t.Fatalf("initial history overwrote newer forwarded ownership: %+v", got)
				}
				if err := ownershipWriterForward(t, peer, source, syncOrderWithdrawBody, false); err != nil {
					t.Fatal(err)
				}
				if got := parseWireUpdates(t, conn.written()); len(got) != 2 || !got[1].withdraws {
					t.Fatalf("newer source lost withdrawal authority: %+v", got)
				}
			default:
				if len(conn.written()) != 0 {
					t.Fatal("stale or unowned initial replay reached the wire")
				}
			}
		})
	}
}

// TestFinalWriterSentReceipt observes the real sent callback after final
// encoding strips ingress ADD-PATH. Its owned sidecar survives transaction
// reuse, and ordinals span legacy and MP announcements of the same family.
func TestFinalWriterSentReceipt(t *testing.T) {
	peer, conn, _ := ownershipWriterPeer(t)
	s := peer.session
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source := makeForwardSourcePeer(t, ctx, ctxID)
	p := syncOrderPrefixWire
	q := []byte{24, 198, 51, 100}
	srcP := append([]byte{0, 0, 0, 0}, p...)
	srcQ := append([]byte{0, 0, 0, 7}, q...)
	// RFC 7911 Section 3 and RFC 4760 Section 3.
	srcAttrs := append(append([]byte(nil), syncOrderAnnounceBody[4:24]...), buildMPReachSource(1, 1, []byte{10, 0, 0, 1}, srcQ)...)
	dstAttrs := append(append([]byte(nil), syncOrderAnnounceBody[4:24]...), buildMPReachSource(1, 1, []byte{10, 0, 0, 1}, q)...)
	sourceBody := fwdPackUpdateBody(&message.Update{PathAttributes: srcAttrs, NLRI: srcP})
	finalBody := fwdPackUpdateBody(&message.Update{PathAttributes: dstAttrs, NLRI: p})
	item := fwdItem{peer: peer, session: s, rawBodies: [][]byte{finalBody}, receivedPeer: source,
		receivedGeneration: source.forwardGeneration.Load(), authority: adjOutForwarded}
	sourceWire := wireu.NewWireUpdate(sourceBody, ctxID)
	if err := prepareFwdProvenance(&item, sourceWire, sourceWire, false); err != nil {
		t.Fatal(err)
	}
	defer item.provenance.release()
	var receipts []wireu.SentOrigin
	var callbackErr string
	s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte, receipt *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection, _ BufHandle, meta map[string]any, sourcePeer string, _ uint64) bool {
		if direction != rpc.DirectionSent || receipt.MessageID() == 0 {
			callbackErr = "sent callback lacks its accepted-message receipt"
		}
		if len(receipts) == 0 && (sourcePeer != source.addrString || receipt.SourceID() != source.sourceID) {
			callbackErr = "sent callback lost body-common source identity"
		}
		if meta != nil {
			callbackErr = "ordinary sent ownership introduced a generic metadata map"
		}
		origin := receipt.SentOrigin()
		if origin == nil {
			callbackErr = "sent callback lacks typed ownership receipt"
			return false
		}
		receipts = append(receipts, *origin)
		return false
	}
	s.writeMu.Lock()
	s.sentForward = &item
	err = s.writeRawUpdateBody(finalBody)
	if err == nil {
		err = s.flushWrites()
	}
	s.sentForward = nil
	s.writeMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	// RFC 4271 Section 4.3: a later local writer reuses the pooled transaction.
	if err := s.SendUpdate(ownershipWriterUpdate(t, syncOrderAnnounceBody)); err != nil {
		t.Fatal(err)
	}
	if callbackErr != "" {
		t.Fatal(callbackErr)
	}
	if len(updateBodies(t, conn.written())) != 2 || len(receipts) != 2 {
		t.Fatal("receipt fixture did not produce both actual UPDATEs")
	}
	paths := receipts[0].Paths
	if len(paths) != 2 {
		t.Fatalf("sent callback lost transformed ingress paths: %+v", receipts[0])
	}
	for i, id := range []uint32{0, 7} {
		if paths[i].Family != family.IPv4Unicast || paths[i].Ordinal != uint32(i) || paths[i].PathID != id {
			t.Fatalf("source-path sidecar changed or has wrong ordinal: %+v", paths)
		}
	}
	if receipts[0].SourceOwner != source.sourceOwner || receipts[0].Local {
		t.Fatal("forwarded receipt was relabeled local")
	}
	if !receipts[1].Local || receipts[1].SourceOwner != 0 || len(receipts[1].Paths) != 0 {
		t.Fatal("local replacement retained forwarded receipt fields")
	}
}
