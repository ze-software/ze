// Design: docs/guide/l2tp.md -- the NCP lifetime peers' own discrimination.

package fixture

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

// ncpLifetimeLoopback answers a peer whose socket talks to a second socket
// standing in for ze, and that second socket. Both close with the test.
func ncpLifetimeLoopback(t *testing.T) (*tunnelAccountingPeer, *net.UDPConn) {
	t.Helper()
	loopback := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	ze, err := net.ListenUDP("udp4", loopback)
	if err != nil {
		t.Fatalf("listen for ze: %v", err)
	}
	t.Cleanup(func() { _ = ze.Close() })
	conn, err := net.ListenUDP("udp4", loopback)
	if err != nil {
		t.Fatalf("listen for the peer: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	target, ok := ze.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("ze's address %v is not UDP", ze.LocalAddr())
	}
	return &tunnelAccountingPeer{conn: conn, target: target, localTID: 7, zeSID: 9, ns: 1, nr: 1}, ze
}

// ncpLifetimeData is one L2TP data message from ze carrying a PPP frame of
// protocol: flags and version 2, Tunnel ID, Session ID, then the frame.
func ncpLifetimeData(protocol uint16, information []byte) []byte {
	packet := make([]byte, 8+len(information))
	binary.BigEndian.PutUint16(packet[0:2], 2)
	binary.BigEndian.PutUint16(packet[2:4], 7)
	binary.BigEndian.PutUint16(packet[4:6], 9)
	binary.BigEndian.PutUint16(packet[6:8], protocol)
	copy(packet[8:], information)
	return packet
}

// ncpLifetimeRead reads what the peer sent ze.
func ncpLifetimeRead(t *testing.T, ze *net.UDPConn) []byte {
	t.Helper()
	buffer := make([]byte, 2048)
	if err := ze.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	n, _, err := ze.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("ze received nothing from the peer: %v", err)
	}
	return buffer[:n]
}

// TestNCPLifetimePeerRejectsIPCPOnce proves the protocol-reject peer can fail.
// Method: hand it ze's IPCP Configure-Request, read the LCP Protocol-Reject it
// sends, byte for byte, then hand it a second IPCP packet and require the
// error that makes the .ci red. A peer that accepted the second packet would
// let a ze that ignores the Protocol-Reject pass.
func TestNCPLifetimePeerRejectsIPCPOnce(t *testing.T) {
	peer, ze := ncpLifetimeLoopback(t)
	peer.rejectIPCP = true
	request := tunnelPPPControl(1, 4, []byte{3, 6, 10, 99, 9, 1})
	if err := peer.handleAccountingPacket(ncpLifetimeData(0x8021, request)); err != nil {
		t.Fatalf("first IPCP packet: %v", err)
	}
	protocol, information, ok := tunnelL2TPPPPData(ncpLifetimeRead(t, ze))
	if !ok || protocol != 0xc021 {
		t.Fatalf("the peer sent protocol %#x (decoded %t), want an LCP frame", protocol, ok)
	}
	// Code 8, Identifier 1, Length 4+2+10, Rejected-Protocol 0x8021, then
	// the whole rejected IPCP packet as the Rejected-Information.
	want := append([]byte{8, 1, 0, 16, 0x80, 0x21}, request...)
	if !bytes.Equal(information, want) {
		t.Fatalf("Protocol-Reject = %x, want %x", information, want)
	}
	if peer.ipcpRejectedAt.IsZero() {
		t.Fatal("the peer did not record when it sent the Protocol-Reject")
	}
	err := peer.handleAccountingPacket(ncpLifetimeData(0x8021, tunnelPPPControl(1, 5, []byte{3, 6, 10, 99, 9, 1})))
	if err == nil || !strings.Contains(err.Error(), "(Code 1, Identifier 5)") || !strings.Contains(err.Error(), "after the LCP Protocol-Reject naming 0x8021") {
		t.Fatalf("second IPCP packet answered %v, want the after-Protocol-Reject failure", err)
	}
}

// TestNCPLifetimePeerKeepsAwaitedCDN proves the CDN is the end only when the
// peer awaits it. Method: the same CDN reaches a peer that does not await one,
// which must fail, and one that does, which must keep its AVPs and send ze the
// ZLB that acknowledges it.
func TestNCPLifetimePeerKeepsAwaitedCDN(t *testing.T) {
	cdn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(tunnelL2TPMessageTypeCDN)), tunnelL2TPAVP(true, 1, tunnelL2TPU16(2))...)
	cdn = append(cdn, tunnelL2TPAVP(true, tunnelL2TPAVPAssignedSessionID, tunnelL2TPU16(9))...)

	unexpected, _ := ncpLifetimeLoopback(t)
	if err := unexpected.handleAccountingPacket(tunnelL2TPControl(700, 7, 1, 2, cdn)); err == nil {
		t.Fatal("a peer awaiting no CDN accepted one")
	}

	peer, ze := ncpLifetimeLoopback(t)
	peer.cdnAwaited = true
	if err := peer.handleAccountingPacket(tunnelL2TPControl(700, 7, 1, 2, cdn)); err != nil {
		t.Fatalf("awaited CDN: %v", err)
	}
	if peer.cdn == nil || !bytes.Equal(peer.cdn[tunnelL2TPAVPAssignedSessionID], []byte{0, 9}) {
		t.Fatalf("the peer kept CDN AVPs %v, want the Assigned Session ID 9", peer.cdn)
	}
	zlb := ncpLifetimeRead(t, ze)
	if len(zlb) != 12 || binary.BigEndian.Uint16(zlb[10:12]) != 2 {
		t.Fatalf("the peer answered %x, want a 12-octet ZLB acknowledging Ns 1 with Nr 2", zlb)
	}
	if err := peer.judgeNoNCPCDN(time.Now(), time.Second); err == nil {
		t.Fatal("judgeNoNCPCDN accepted a CDN that followed no Protocol-Reject")
	}
}

// TestNCPLifetimeJudgeRefusesAShortWindow proves the protocol-reject peer
// refuses a window in which ze owed no retransmission, and accepts the CDN
// the test expects. Method: one Protocol-Reject recorded half a Restart time
// before the CDN is refused; one recorded past it passes.
func TestNCPLifetimeJudgeRefusesAShortWindow(t *testing.T) {
	peer, _ := ncpLifetimeLoopback(t)
	peer.cdn = map[uint16][]byte{1: {0, 2}, tunnelL2TPAVPAssignedSessionID: {0, 9}}
	now := time.Now()
	peer.ipcpRejectedAt = now.Add(-time.Second)
	if err := peer.judgeNoNCPCDN(now, 2*time.Second); err == nil || !strings.Contains(err.Error(), "inside one Restart time") {
		t.Fatalf("a CDN one second after the Protocol-Reject with a 2s Restart time answered %v, want a refusal", err)
	}
	peer.ipcpRejectedAt = now.Add(-3 * time.Second)
	if err := peer.judgeNoNCPCDN(now, 2*time.Second); err != nil {
		t.Fatalf("a CDN three seconds after the Protocol-Reject: %v", err)
	}
	peer.cdn[1] = []byte{0, 3}
	if err := peer.judgeNoNCPCDN(now, 2*time.Second); err == nil {
		t.Fatal("a CDN with Result Code 3 was accepted")
	}
}

// TestNCPLifetimeArgDurationRefuses proves a bad bound argument stops the
// peer before any session exists, rather than turning into a zero hold.
func TestNCPLifetimeArgDurationRefuses(t *testing.T) {
	for _, args := range [][]string{{"1701"}, {"1701", "x"}, {"1701", "0"}, {"1701", "-3"}} {
		if _, err := ncpLifetimeArgDuration(args, 1, time.Second, "ncp timeout seconds"); err == nil {
			t.Fatalf("args %q were accepted", args)
		}
	}
	got, err := ncpLifetimeArgDuration([]string{"1701", "15"}, 1, time.Second, "ncp timeout seconds")
	if err != nil || got != 15*time.Second {
		t.Fatalf("15 seconds read as %s, %v", got, err)
	}
}
