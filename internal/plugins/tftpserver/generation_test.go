package tftpserver

import (
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestTransferGenerationEndClosesBeforeReturning proves the cut a reload
// relies on is synchronous: when end returns, the socket of every transfer
// that joined is already closed, a later join is refused, and leave tells
// the transfer that end owns the close.
// Method: join one UDP socket, end the generation, and write on the socket
// at once, with no wait that would let an asynchronous close catch up.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 25 (cut on change).
// PREVENTS: a block sent from the replaced root after the reload returned,
// which the context.AfterFunc close allowed (TestConfigTxReloadCutsTransferInFlight
// failed 3 runs in 20 under -race).
func TestTransferGenerationEndClosesBeforeReturning(t *testing.T) {
	generation := newTransferGeneration()
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if !generation.join(conn) {
		t.Fatal("a running generation refused a transfer")
	}

	generation.end(slogutil.DiscardLogger())

	if _, err := conn.Write([]byte{0}); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("write after end answered %v, want net.ErrClosed: the transfer socket was still open", err)
	}
	if generation.leave(conn) {
		t.Fatal("leave after end said the transfer still owns a socket end closed")
	}
	late, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = late.Close() })
	if generation.join(late) {
		t.Fatal("an ended generation accepted a transfer")
	}
}

// TestTransferGenerationLeaveHandsBackTheClose proves a transfer that ends
// on its own keeps the close: leave answers true once, and a later end does
// not close the socket a second time.
// VALIDATES: the join/leave pair in handleRRQ closes every socket once.
// PREVENTS: a transfer socket leaked, or closed by both sides.
func TestTransferGenerationLeaveHandsBackTheClose(t *testing.T) {
	generation := newTransferGeneration()
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if !generation.join(conn) {
		t.Fatal("a running generation refused a transfer")
	}
	if !generation.leave(conn) {
		t.Fatal("leave of a joined transfer did not hand back the close")
	}
	generation.end(slogutil.DiscardLogger())
	if _, err := conn.Write([]byte{0}); err != nil {
		t.Fatalf("end closed a socket that had left: %v", err)
	}
}
