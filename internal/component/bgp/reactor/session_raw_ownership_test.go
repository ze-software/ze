// Design: docs/architecture/update-building.md -- explicit raw injection and final ownership.
// Related: session_write.go -- real buffered writes, flush and exact-session retirement.
package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rawOwnershipConn observes flush-before-close on the synchronous writer rail.
// The optional hook installs a replacement while the old connection is flushing.
type rawOwnershipConn struct {
	recordingConn
	closed     bool
	closeBytes []byte
	writeError error
	afterWrite func()
}

func (c *rawOwnershipConn) Write(data []byte) (int, error) {
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	if c.writeError != nil {
		return 0, c.writeError
	}
	n, err := c.recordingConn.Write(data)
	if hook := c.afterWrite; hook != nil {
		c.afterWrite = nil
		hook()
	}
	return n, err
}

func (c *rawOwnershipConn) Close() error {
	c.closed = true
	c.closeBytes = c.written()
	return nil
}

func rawOwnershipPeer(t *testing.T) (*Peer, *rawOwnershipConn) {
	t.Helper()
	peer, _ := newAttachedPeer(t, railDest, sendRawOnly(railGranted))
	peer.session.localOpen = &message.Open{MyAS: 65000, HoldTime: 90}
	peer.session.peerOpen = &message.Open{MyAS: 65001, HoldTime: 90}
	peer.session.negotiateWith(nil, nil)
	peer.setEncodingContexts(peer.session.Negotiated())
	peer.session.adjOut = &peer.adjOut
	conn := &rawOwnershipConn{}
	peer.session.conn = conn
	peer.session.bufWriter = bufio.NewWriterSize(conn, 4096)
	t.Cleanup(peer.clearEncodingContexts)
	return peer, conn
}

// rawOwnershipPacket preserves arbitrary bodies while adding the standard header.
// RFC 4271 Section 4.1: use the production header writer for the fixture framing.
func rawOwnershipPacket(body []byte, kind msgtype.MessageType) []byte {
	packet := make([]byte, message.HeaderLen+len(body))
	header := message.Header{Length: uint16(len(packet)), Type: kind}
	header.WriteTo(packet, 0)
	copy(packet[message.HeaderLen:], body)
	return packet
}

// TestRawOpaqueInjectionFlushesBeforeRetirement drives the authorized API with
// malformed bodies and framing. Accepted diagnostic bytes must leave before the
// exact Session retires; they must not create sent ownership or AIGP receipts.
func TestRawOpaqueInjectionFlushesBeforeRetirement(t *testing.T) {
	bad := []byte{0xde, 0xad, 0xbe, 0xef}
	keepalive := rawOwnershipPacket(nil, msgtype.TypeKEEPALIVE)
	trailing := append(bytes.Clone(keepalive), rawOwnershipPacket(syncOrderAnnounceBody, msgtype.TypeUPDATE)...)
	badMarker := bytes.Clone(keepalive)
	badMarker[0] = 0
	mixedTail := append(bytes.Clone(syncOrderAnnounceBody), 33)
	for _, tc := range []struct {
		name    string
		kind    uint8
		payload []byte
		want    []byte
	}{
		{"body", uint8(msgtype.TypeUPDATE), bad, rawOwnershipPacket(bad, msgtype.TypeUPDATE)},
		{"packet", 0, rawOwnershipPacket(bad, msgtype.TypeUPDATE), rawOwnershipPacket(bad, msgtype.TypeUPDATE)},
		{"attribute-tail", uint8(msgtype.TypeUPDATE), []byte{0, 0, 0, 1, 0x40}, rawOwnershipPacket([]byte{0, 0, 0, 1, 0x40}, msgtype.TypeUPDATE)},
		{"nlri-tail", uint8(msgtype.TypeUPDATE), []byte{0, 0, 0, 0, 33}, rawOwnershipPacket([]byte{0, 0, 0, 0, 33}, msgtype.TypeUPDATE)},
		{"valid-path-before-nlri-tail", uint8(msgtype.TypeUPDATE), mixedTail, rawOwnershipPacket(mixedTail, msgtype.TypeUPDATE)},
		{"control-trailing-update", 0, trailing, trailing},
		{"bad-marker", 0, badMarker, badMarker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := rawOwnershipPeer(t)
			s := peer.session
			var receipts int
			s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
				receipts++
				return false
			}
			api := newSendPermissionReactor(peer)
			if err := api.SendRawMessage(peer.settings.Address, tc.kind, tc.payload, plugin.ProcessSender(railGranted)); err != nil {
				t.Fatalf("accepted opaque injection returned a transport failure: %v", err)
			}
			if !bytes.Equal(conn.written(), tc.want) || !bytes.Equal(conn.closeBytes, tc.want) {
				t.Fatalf("opaque bytes were changed or closed before flush: written=%x at-close=%x want=%x", conn.written(), conn.closeBytes, tc.want)
			}
			if !conn.closed || !s.tearingDown.Load() {
				t.Fatal("opaque output left its Session reusable")
			}
			cause := s.closeReason.Load()
			if cause == nil || !errors.Is(*cause, errRawOwnership) {
				t.Fatalf("retirement did not identify successful opaque injection: %v", cause)
			}
			if receipts != 0 || len(s.aigpPending) != 0 || len(peer.adjOut.routes) != 0 {
				t.Fatal("opaque bytes manufactured ownership or AIGP receipts")
			}
			before := len(conn.written())
			if err := s.SendUpdate(ownershipWriterUpdate(t, syncOrderAnnounceBody)); !errors.Is(err, errRawOwnership) {
				t.Fatalf("retired managed writer = %v", err)
			}
			if err := s.SendRawMessage(uint8(msgtype.TypeKEEPALIVE), nil); !errors.Is(err, errRawOwnership) {
				t.Fatalf("retired raw writer = %v", err)
			}
			if len(conn.written()) != before {
				t.Fatal("retired session emitted more bytes")
			}
		})
	}
}

// TestRawValidDuplicatesOwnTheLocalPath proves both API forms emit identical
// requests twice and replace a real forwarded owner, without weakening the
// remote-source withdrawal guard. Full UPDATE headers retain legacy rebuilding.
func TestRawValidDuplicatesOwnTheLocalPath(t *testing.T) {
	for _, packet := range []bool{false, true} {
		name := "body"
		if packet {
			name = "packet"
		}
		t.Run(name, func(t *testing.T) {
			peer, conn := rawOwnershipPeer(t)
			source, _ := newAnnouncePeer(t, "192.0.2.3")
			if err := ownershipWriterForward(t, peer, source, syncOrderAnnounceBody, false); err != nil {
				t.Fatal(err)
			}
			var localReceipts int
			peer.session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte, receipt *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
				localReceipts++
				if receipt == nil {
					t.Error("raw advertisement has no sent receipt")
					return false
				}
				origin := receipt.SentOrigin()
				if origin == nil {
					t.Error("raw advertisement has no ownership receipt")
					return false
				}
				if !origin.Local {
					t.Error("raw advertisement retained forwarded ownership")
				}
				return false
			}
			kind, payload := uint8(msgtype.TypeUPDATE), syncOrderAnnounceBody
			if packet {
				kind, payload = 0, rawOwnershipPacket(syncOrderAnnounceBody, msgtype.TypeUPDATE)
				payload[0] = 0
				binary.BigEndian.PutUint16(payload[16:18], message.HeaderLen)
			}
			api := newSendPermissionReactor(peer)
			for range 2 {
				if err := api.SendRawMessage(peer.settings.Address, kind, payload, plugin.ProcessSender(railGranted)); err != nil {
					t.Fatal(err)
				}
			}
			if err := ownershipWriterForward(t, peer, source, syncOrderWithdrawBody, false); err != nil {
				t.Fatal(err)
			}
			if localReceipts != 2 {
				t.Fatalf("local raw receipts = %d, want 2", localReceipts)
			}
			bodies := updateBodies(t, conn.written())
			if len(bodies) != 3 {
				t.Fatalf("raw duplicate suppressed or remote withdrawal escaped: %d UPDATEs", len(bodies))
			}
			for _, body := range bodies {
				if !bytes.Equal(body, syncOrderAnnounceBody) {
					t.Fatalf("raw advertisement changed: %x", body)
				}
			}
			if conn.closed || peer.session.tearingDown.Load() {
				t.Fatal("accountable raw output retired its Session")
			}
		})
	}
}

// TestRawOpaqueRetirementKeepsReplacement exercises pending managed output and
// installs a new Session during the old flush. The old safety retirement must
// not clear the replacement's frontier or prevent its legitimate withdrawal.
func TestRawOpaqueRetirementKeepsReplacement(t *testing.T) {
	peer, oldConn := rawOwnershipPeer(t)
	old := peer.session
	old.writeMu.Lock()
	err := old.writeRawUpdateBody(syncOrderAnnounceBody)
	old.writeMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if len(oldConn.written()) != 0 {
		t.Fatal("pending-frontier setup flushed early")
	}
	replacement, nextConn := rawOwnershipPeer(t)
	next := replacement.session
	next.adjOut = &peer.adjOut
	oldConn.afterWrite = func() {
		peer.mu.Lock()
		peer.session = next
		peer.mu.Unlock()
		if err := next.SendUpdate(ownershipWriterUpdate(t, syncOrderAnnounceBody)); err != nil {
			t.Fatal(err)
		}
	}
	bad := []byte{0xde, 0xad, 0xbe, 0xef}
	if err := old.SendRawMessage(uint8(msgtype.TypeUPDATE), bad); err != nil {
		t.Fatal(err)
	}
	want := append(rawOwnershipPacket(syncOrderAnnounceBody, msgtype.TypeUPDATE), rawOwnershipPacket(bad, msgtype.TypeUPDATE)...)
	if !bytes.Equal(oldConn.closeBytes, want) {
		t.Fatalf("pending output was lost or closed before flush: %x", oldConn.closeBytes)
	}
	if !oldConn.closed || nextConn.closed || next.tearingDown.Load() || peer.adjOut.session != next || len(peer.adjOut.routes) != 1 {
		t.Fatal("old opaque retirement damaged the replacement")
	}
	if err := old.SendRawMessage(uint8(msgtype.TypeUPDATE), syncOrderAnnounceBody); !errors.Is(err, errRawOwnership) {
		t.Fatalf("old writer re-entered replacement frontier: %v", err)
	}
	if err := next.SendUpdate(ownershipWriterUpdate(t, syncOrderWithdrawBody)); err != nil {
		t.Fatal(err)
	}
	if len(updateBodies(t, nextConn.written())) != 2 {
		t.Fatal("replacement could not advertise and withdraw normally")
	}
}

// TestRawInjectionPreservesPathsLimit proves raw bypasses only the new ownership
// suppression, not the previously negotiated limit or its malformed refusal.
func TestRawInjectionPreservesPathsLimit(t *testing.T) {
	for _, packet := range []bool{false, true} {
		s, conn := pathsLimitSession(t, map[family.Family]uint16{family.IPv4Unicast: 1})
		send := func(body []byte) error {
			if packet {
				return s.SendRawMessage(0, rawOwnershipPacket(body, msgtype.TypeUPDATE))
			}
			return s.SendRawMessage(uint8(msgtype.TypeUPDATE), body)
		}
		first := fwdPackUpdateBody(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(1, "198.51.100.0/24")))
		second := fwdPackUpdateBody(pathsLimitUpdate(family.IPv4Unicast, false, pathsLimitNLRI(2, "198.51.100.0/24")))
		for _, body := range [][]byte{first, first, second} {
			if err := send(body); err != nil {
				t.Fatal(err)
			}
		}
		if got := pathsLimitReceived(t, conn.written(), family.IPv4Unicast, false); len(got) != 2 || got[0] != 1 || got[1] != 1 {
			t.Fatalf("raw changed existing limit policy or suppressed a duplicate: %v", got)
		}
		before := len(conn.written())
		if err := send([]byte{0xde, 0xad, 0xbe, 0xef}); err == nil {
			t.Fatal("raw bypassed the existing PATHS-LIMIT malformed refusal")
		}
		if len(conn.written()) != before || s.tearingDown.Load() {
			t.Fatal("policy-refused raw input emitted bytes or retired the Session")
		}
	}
}

// TestRawOpaqueTransportFailureKeepsItsCause distinguishes unsuccessful flush
// from successful diagnostic injection followed by the local safety reset.
func TestRawOpaqueTransportFailureKeepsItsCause(t *testing.T) {
	peer, conn := rawOwnershipPeer(t)
	conn.writeError = io.ErrClosedPipe
	if err := peer.session.SendRawMessage(uint8(msgtype.TypeUPDATE), []byte{0xde, 0xad, 0xbe, 0xef}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("real transport failure replaced by diagnostic success: %v", err)
	}
	cause := peer.session.closeReason.Load()
	if cause == nil || !errors.Is(*cause, io.ErrClosedPipe) || !conn.closed {
		t.Fatal("failed raw flush lost its actual transport cause")
	}
}

// The explicit raw mode must survive AIGP policy rewriting. A stripped duplicate
// remains an explicit write, while an enabled, permitted metric is retained.
func TestRawInjectionKeepsAIGPPolicy(t *testing.T) {
	for _, packet := range []bool{false, true} {
		mode := "body"
		if packet {
			mode = "packet"
		}
		t.Run(mode, func(t *testing.T) {
			for _, tc := range []struct {
				name      string
				enabled   bool
				originate bool
				want      bool
			}{
				{"disabled", false, true, false},
				{"origin-disabled", true, false, false},
				{"enabled", true, true, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					peer, conn := rawOwnershipPeer(t)
					s := peer.session
					s.settings.AIGPSession = &tc.enabled
					s.settings.AIGPOriginate = tc.originate
					s.settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
					body := aigpTestBody(37)
					kind, payload := uint8(msgtype.TypeUPDATE), body
					if packet {
						kind, payload = 0, rawOwnershipPacket(body, msgtype.TypeUPDATE)
					}
					api := newSendPermissionReactor(peer)
					for range 2 {
						if err := api.SendRawMessage(peer.settings.Address, kind, payload, plugin.ProcessSender(railGranted)); err != nil {
							t.Fatal(err)
						}
					}
					bodies := updateBodies(t, conn.written())
					if len(bodies) != 2 {
						t.Fatalf("AIGP policy suppressed explicit duplicate: %d UPDATEs", len(bodies))
					}
					for _, sent := range bodies {
						metric, present := aigpReceivedMetric(t, sent)
						if present != tc.want {
							t.Fatalf("AIGP present = %v, want %v", present, tc.want)
						}
						if present && metric != 37 {
							t.Fatalf("AIGP metric = %d, want 37", metric)
						}
					}
					if conn.closed {
						t.Fatal("accountable AIGP write retired the session")
					}
				})
			}
		})
	}
}

func TestRawControlInjectionKeepsOwnership(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    uint8
		payload []byte
	}{
		{"empty", 0, nil},
		{"keepalive-body", uint8(msgtype.TypeKEEPALIVE), nil},
		{"keepalive-packet", 0, rawOwnershipPacket(nil, msgtype.TypeKEEPALIVE)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := rawOwnershipPeer(t)
			source, _ := newAnnouncePeer(t, "192.0.2.1")
			if err := ownershipWriterForward(t, peer, source, syncOrderAnnounceBody, false); err != nil {
				t.Fatal(err)
			}
			api := newSendPermissionReactor(peer)
			if err := api.SendRawMessage(peer.settings.Address, tc.kind, tc.payload, plugin.ProcessSender(railGranted)); err != nil {
				t.Fatal(err)
			}
			if conn.closed {
				t.Fatal("control injection retired an accountable session")
			}
			if err := ownershipWriterForward(t, peer, source, syncOrderWithdrawBody, false); err != nil {
				t.Fatal(err)
			}
			bodies := updateBodies(t, conn.written())
			if len(bodies) != 2 {
				t.Fatalf("control injection changed route ownership: %d UPDATEs", len(bodies))
			}
			if !bytes.Equal(bodies[1], syncOrderWithdrawBody) {
				t.Fatalf("owned withdrawal changed after control injection: %x", bodies[1])
			}
		})
	}
}
