// Design: docs/architecture/mrt.md — winning connection OPEN fidelity and epoch isolation.
package reactor

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/mrt"
	mrtplugin "github.com/ze-software/ze/internal/plugins/mrt"
)

// TestMRTWinningCollisionPreservesOPEN follows a live Peer's collision handoff,
// not acceptWithOpen in isolation. Two TCP connections send different actual
// capabilities. The winner must establish, retain its noncanonical OPEN bytes,
// and supply the context for mixed-family UPDATEs in both directions.
func TestMRTWinningCollisionPreservesOPEN(t *testing.T) {
	testMRTWinningCollision(t, "")
}

// TestMRTCollisionWinnerReservation forces third arrivals both while the fresh
// session is published and after it has taken the winner but cannot install it.
// Existing Peer callbacks and the actual Session mutex provide the barriers;
// every subtest then completes the same socket and exact MRT assertions.
func TestMRTCollisionWinnerReservation(t *testing.T) {
	for _, phase := range []string{"published", "taken"} {
		t.Run(phase, func(t *testing.T) {
			testMRTWinningCollision(t, phase)
		})
	}
}

func testMRTWinningCollision(t *testing.T, handoffPhase string) {
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x0a000002)
	settings.LocalAddress = netip.MustParseAddr("127.0.0.1")
	settings.Connection = ConnectionPassive
	settings.Capabilities = []capability.Capability{
		&capability.Multiprotocol{AFI: 1, SAFI: 1},
		&capability.Multiprotocol{AFI: 2, SAFI: 1},
		&capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: 1, SAFI: 1, Mode: capability.AddPathSend},
			{AFI: 2, SAFI: 1, Mode: capability.AddPathReceive},
		}},
	}
	path := filepath.Join(t.TempDir(), "winning-collision.mrt")
	recorder := mrtplugin.New(mrtplugin.Config{AllPath: path}, nil)
	recorder.Start(nil)
	stopRecorder := sync.OnceFunc(recorder.Stop)
	t.Cleanup(stopRecorder)
	r := New(&Config{})
	r.addMessageObserver(recorder)
	// Register through the real reactor path: it owns the semantic callback
	// and peer lookup used by the UPDATE-receipt counter below.
	if err := r.AddPeer(settings); err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	peer := r.peers[settings.PeerKey()]
	r.mu.RUnlock()
	var staged chan *Session
	var resume chan struct{}
	if handoffPhase != "" {
		staged = make(chan *Session, 1)
		resume = make(chan struct{})
		var active atomic.Int32
		peer.SetCallback(func(_, to PeerState) {
			if to != PeerStateActive {
				return
			}
			if active.Add(1) != 2 {
				return
			}
			fresh := peer.currentSession()
			if handoffPhase == "taken" {
				// Start uses the FSM's lock, not Session.mu. runOnce can
				// start and take the slot, but acceptWithOpen must wait.
				fresh.mu.Lock()
				go func() {
					<-resume
					fresh.mu.Unlock()
				}()
				staged <- fresh
				return
			}
			staged <- fresh
			<-resume
		})
	}
	peer.StartWithContext(t.Context())
	stopPeer := sync.OnceFunc(func() {
		peer.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := peer.Wait(ctx); err != nil {
			t.Errorf("stop live peer: %v", err)
		}
	})
	t.Cleanup(stopPeer)
	releaseHandoff := sync.OnceFunc(func() {
		if resume != nil {
			close(resume)
		}
	})
	t.Cleanup(releaseHandoff)
	mrtCollisionWait(t, func() bool { return peer.SessionState() == fsm.StateActive }, "passive session ready")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	dial := func() (net.Conn, net.Conn) {
		t.Helper()
		client, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { client.Close() })
		server, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { server.Close() })
		if err := client.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatal(err)
		}
		return client, server
	}
	connect := func() net.Conn {
		t.Helper()
		client, server := dial()
		r.acceptOrReject(server, peer, nil)
		return client
	}
	write := func(conn net.Conn, wire []byte) {
		t.Helper()
		n, err := conn.Write(wire)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(wire) {
			t.Fatalf("short TCP write: %d/%d", n, len(wire))
		}
	}
	existing := connect()
	oldLocal := readMRTTestPacket(t, existing)
	if oldLocal[18] != 1 {
		t.Fatalf("initial reply is not OPEN: %x", oldLocal)
	}
	oldRemote := mrtCollisionOPEN(2, 1)
	write(existing, oldRemote)
	if packet := readMRTTestPacket(t, existing); packet[18] != 4 {
		t.Fatalf("initial OPEN was not accepted: %x", packet)
	}
	mrtCollisionWait(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "original OpenConfirm")

	winner := connect()
	winningRemote := mrtCollisionOPEN(1, 2)
	write(winner, winningRemote)
	cease := readMRTTestPacket(t, existing)
	if cease[18] != 3 || !bytes.Equal(cease[19:], []byte{6, 7}) {
		t.Fatalf("losing connection did not receive collision Cease: %x", cease)
	}
	if handoffPhase != "" {
		select {
		case fresh := <-staged:
			if peer.currentSession() != fresh {
				t.Fatal("handoff barrier did not retain the actual published session")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("replacement session did not reach the handoff barrier")
		}
		if handoffPhase == "taken" {
			mrtCollisionWait(t, func() bool {
				peer.mu.RLock()
				empty := peer.inbound.conn == nil
				peer.mu.RUnlock()
				return empty && peer.SessionState() == fsm.StateActive
			}, "winner taken before socket installation")
		}
		third, server := dial()
		accepted := make(chan struct{})
		go func() {
			r.acceptOrReject(server, peer, nil)
			close(accepted)
		}()
		select {
		case <-accepted:
		case <-time.After(5 * time.Second):
			t.Fatal("third arrival entered the reserved Session accept path")
		}
		var octet [1]byte
		n, err := third.Read(octet[:])
		if n != 0 || err != io.EOF {
			t.Fatalf("third arrival was not refused before OPEN: read=%d error=%v byte=%x", n, err, octet)
		}
		releaseHandoff()
	}
	// This must be the same surviving TCP connection, not a retry. A lost
	// handoff fails here (EOF/timeout) instead of being hidden by a new dial.
	winningLocal := readMRTTestPacket(t, winner)
	if winningLocal[18] != 1 {
		t.Fatalf("winning connection did not receive OPEN: %x", winningLocal)
	}
	if packet := readMRTTestPacket(t, winner); packet[18] != 4 {
		t.Fatalf("winning OPEN was not accepted: %x", packet)
	}
	keepalive := buildUpdateMsg(nil)
	keepalive[18] = 4
	write(winner, keepalive)
	mrtCollisionWait(t, func() bool { return peer.State() == PeerStateEstablished }, "winning Established")

	incoming := buildUpdateMsg([]byte{0, 4, 24, 10, 1, 0, 0, 15, 0x80, 15, 12, 0, 2, 1, 1, 2, 3, 4, 32, 0x20, 1, 0x0d, 0xb9})
	write(winner, incoming)
	mrtCollisionWait(t, func() bool { return peer.Stats().UpdatesReceived == 1 }, "winning UPDATE receipt")
	outgoing := buildUpdateMsg([]byte{0, 8, 5, 6, 7, 8, 24, 10, 2, 0, 0, 11, 0x80, 15, 8, 0, 2, 1, 32, 0x20, 1, 0x0d, 0xba})
	if err := peer.SendRawMessage(0, outgoing); err != nil {
		t.Fatal(err)
	}
	// Initial EORs may precede this UPDATE. Bound the walk and require the
	// exact outgoing bytes rather than treating an arbitrary UPDATE as ours.
	found := false
	for range 8 {
		if bytes.Equal(readMRTTestPacket(t, winner), outgoing) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("winning socket never received the exact outbound UPDATE")
	}
	stopPeer()
	stopRecorder()

	var opens [][]byte
	var directions []uint16
	received, sent := 0, 0
	err = mrt.ReadFile(path, &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, record *mrt.MessageRecord) error {
		wire := record.BGPMessage.Bytes
		if wire[18] == 1 {
			opens = append(opens, bytes.Clone(wire))
			directions = append(directions, h.Subtype)
			return nil
		}
		if wire[18] != 2 {
			return nil
		}
		parsed, err := mrt.ParseBGPMessage(record.BGPMessage)
		if err != nil {
			return err
		}
		u := parsed.Update
		if !bytes.Equal(wire, incoming) && !bytes.Equal(wire, outgoing) {
			// Only harmless initial EOR messages are allowed in addition to
			// the two discriminating UPDATEs supplied above.
			if len(u.WithdrawnPrefixes)+len(u.AnnouncedPrefixes) != 0 {
				t.Fatalf("unexpected recorded routes: %x", wire)
			}
			return nil
		}
		mp, err := mrt.ParseMPUnreach(mrt.FindAttribute(u.Attributes, 15).Value, u.AddPathFor(2, 1))
		if err != nil {
			return err
		}
		wantClassic, wantMP := "10.1.0.0/24", "2001:db9::/32"
		if bytes.Equal(wire, incoming) {
			received++
			if h.Subtype != mrt.BGP4MPMessageAS4AP || len(u.WithdrawnPathIDs) != 0 || !slices.Equal(mp.PathIDs, []uint32{0x01020304}) {
				t.Fatalf("incoming winner context: subtype=%d classic IDs=%x MP IDs=%x", h.Subtype, u.WithdrawnPathIDs, mp.PathIDs)
			}
		} else {
			sent++
			wantClassic, wantMP = "10.2.0.0/24", "2001:dba::/32"
			if h.Subtype != mrt.BGP4MPMessageAS4LocalAP || !slices.Equal(u.WithdrawnPathIDs, []uint32{0x05060708}) || len(mp.PathIDs) != 0 {
				t.Fatalf("outgoing winner context: subtype=%d classic IDs=%x MP IDs=%x", h.Subtype, u.WithdrawnPathIDs, mp.PathIDs)
			}
		}
		if !slices.Equal(u.WithdrawnPrefixes, []netip.Prefix{netip.MustParsePrefix(wantClassic)}) || !slices.Equal(mp.Prefixes, []netip.Prefix{netip.MustParsePrefix(wantMP)}) {
			t.Fatalf("winner prefixes=%v/%v", u.WithdrawnPrefixes, mp.Prefixes)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	wantOpens := [][]byte{oldLocal, oldRemote, winningLocal, winningRemote}
	if !slices.EqualFunc(opens, wantOpens, bytes.Equal) || !slices.Equal(directions, []uint16{7, 4, 7, 4}) {
		t.Fatalf("original directional OPENs changed or disappeared: directions=%v OPENs=%x", directions, opens)
	}
	if received != 1 || sent != 1 {
		t.Fatalf("winning UPDATE records received/sent=%d/%d, want 1/1", received, sent)
	}
}

// mrtCollisionOPEN uses multiple capability parameters and an unknown capability
// so rebuilding an OPEN from negotiated settings cannot satisfy byte equality.
func mrtCollisionOPEN(mode4, mode6 byte) []byte {
	params := []byte{
		2, 6, 65, 4, 0, 0, 0xfd, 0xe9,
		2, 6, 1, 4, 0, 2, 0, 1,
		2, 5, 222, 3, 0x91, 0x82, 0x73,
		2, 6, 1, 4, 0, 1, 0, 1,
		2, 10, 69, 8, 0, 1, 1, mode4, 0, 2, 1, mode6,
	}
	body := []byte{4, 0xfd, 0xe9, 0, 90, 10, 0, 0, 3, byte(len(params))}
	wire := buildUpdateMsg(append(body, params...))
	wire[18] = 1
	return wire
}

func mrtCollisionWait(t *testing.T, ready func() bool, boundary string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for %s", boundary)
		case <-tick.C:
		}
	}
}
