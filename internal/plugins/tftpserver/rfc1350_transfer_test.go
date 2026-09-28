// VALIDATES: RFC 1350 transfer behavior of the read path, observed from a
// client on loopback: lockstep, duplicate ACKs, the server TID, end-of-transfer
// framing, retransmission and the abort on timeout.
// PREVENTS: a server that streams ahead of the ACKs, answers a duplicate ACK
// with DATA (the Sorcerer's Apprentice bug), keeps sending after the final
// block, or retransmits forever.
package tftpserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// quietWindow is how long a test waits to prove that the server sends nothing.
// The server answers an ACK at once, so a wrong packet arrives well inside it.
const quietWindow = time.Second

// transferClient drives one TFTP read from the client side of a loopback
// transfer. It is used by one test goroutine only.
type transferClient struct {
	t        *testing.T
	conn     *net.UDPConn
	listener *net.UDPAddr // the server's request port
	tid      *net.UDPAddr // the server's transfer TID, learned from its first reply
	buf      []byte       // large enough that an oversize datagram is read whole
}

// newTransferClient starts a server with ten transfer slots over a root
// holding files and returns a client bound to a free loopback port.
func newTransferClient(t *testing.T, files map[string][]byte) *transferClient {
	t.Helper()

	rootDir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(rootDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	listener := startTestTFTPServer(t, rootDir, 10)
	return newTransferClientFor(t, listener)
}

// newTransferClientFor returns a client for an already running server.
func newTransferClientFor(t *testing.T, listener *net.UDPAddr) *transferClient {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeUDP(t, conn) })

	return &transferClient{t: t, conn: conn, listener: listener, buf: make([]byte, 65536+4)}
}

// sendRRQ sends an octet RRQ for filename to the listener, with option pairs.
func (c *transferClient) sendRRQ(filename string, opts ...string) {
	c.t.Helper()
	if _, err := c.conn.WriteToUDP(buildRRQPacketWithOptions(filename, opts...), c.listener); err != nil {
		c.t.Fatal(err)
	}
}

// ack sends an ACK of block to the server's transfer TID.
func (c *transferClient) ack(block uint16) {
	c.t.Helper()
	if c.tid == nil {
		c.t.Fatal("ACK before the server chose a TID")
	}
	if _, err := c.conn.WriteToUDP(buildACKPacket(block), c.tid); err != nil {
		c.t.Fatal(err)
	}
}

// read waits up to wait for one datagram. It returns a copy of the datagram,
// its source, and false on timeout.
func (c *transferClient) read(wait time.Duration) ([]byte, *net.UDPAddr, bool) {
	c.t.Helper()
	if err := c.conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		c.t.Fatal(err)
	}
	n, source, err := c.conn.ReadFromUDP(c.buf)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, nil, false
		}
		c.t.Fatal(err)
	}
	if c.tid == nil {
		c.tid = source
	}
	return bytes.Clone(c.buf[:n]), source, true
}

// expectData requires the next datagram to be DATA of block and returns its
// data field.
func (c *transferClient) expectData(block uint16, wait time.Duration) []byte {
	c.t.Helper()
	pkt, _, ok := c.read(wait)
	if !ok {
		c.t.Fatalf("no DATA block %d within %v", block, wait)
	}
	requireData(c.t, pkt, block)
	return pkt[4:]
}

// expectSilence requires that no datagram arrives within wait.
func (c *transferClient) expectSilence(wait time.Duration, why string) {
	c.t.Helper()
	pkt, _, ok := c.read(wait)
	if !ok {
		return
	}
	if len(pkt) >= 4 {
		c.t.Fatalf("%s: got opcode %d block %d (%d data octets)",
			why, binary.BigEndian.Uint16(pkt[0:2]), binary.BigEndian.Uint16(pkt[2:4]), len(pkt)-4)
	}
	c.t.Fatalf("%s: got a %d-octet datagram", why, len(pkt))
}

// requireData fails unless pkt is a DATA packet of block.
func requireData(t *testing.T, pkt []byte, block uint16) {
	t.Helper()
	if len(pkt) < 4 {
		t.Fatalf("datagram of %d octets, want DATA block %d", len(pkt), block)
	}
	if op := binary.BigEndian.Uint16(pkt[0:2]); op != opDATA {
		t.Fatalf("opcode %d, want DATA block %d", op, block)
	}
	if got := binary.BigEndian.Uint16(pkt[2:4]); got != block {
		t.Fatalf("DATA block %d, want %d", got, block)
	}
}

// patterned returns n octets that differ from their neighbors, so a misplaced
// block is visible.
func patterned(n int) []byte {
	content := make([]byte, n)
	for i := range content {
		content[i] = byte(i % 251)
	}
	return content
}

// RFC requirement: RFC1350-2-2 positive -- DATA block 1 carries one 512-octet block of the
// file, and once the client ACKs block 1 the server sends block 2 carrying the next block.
// RFC requirement: RFC1350-2-2 negative -- while block 1 is unacknowledged the server sends
// no block 2: nothing arrives for one second after block 1 (the ACK timeout is 5 s).
func TestRFC1350LockstepWaitsForTheACK(t *testing.T) {
	t.Parallel()

	content := patterned(1500)
	c := newTransferClient(t, map[string][]byte{"f.bin": content})
	c.sendRRQ("f.bin")

	first := c.expectData(1, 2*time.Second)
	if !bytes.Equal(first, content[:512]) {
		t.Fatalf("block 1 carries %d octets, want the first 512 of the file", len(first))
	}

	c.expectSilence(quietWindow, "DATA sent before block 1 was acknowledged")

	c.ack(1)
	second := c.expectData(2, 2*time.Second)
	if !bytes.Equal(second, content[512:1024]) {
		t.Fatalf("block 2 carries %d octets, want octets 512..1023 of the file", len(second))
	}
}

// RFC requirement: RFC1350-2-3 positive -- a fresh ACK is acknowledged by the next DATA:
// after ACK 2 the server sends block 3, exactly once.
// RFC requirement: RFC1350-2-3 negative -- a duplicate ACK is not acknowledged: a second
// ACK 1, sent while block 2 is outstanding, produces no DATA for one second, so the
// server does not resend block 2 on it (the Sorcerer's Apprentice fix).
func TestRFC1350DuplicateACKTriggersNoDATA(t *testing.T) {
	t.Parallel()

	c := newTransferClient(t, map[string][]byte{"f.bin": patterned(1500)})
	c.sendRRQ("f.bin")

	c.expectData(1, 2*time.Second)
	c.ack(1)
	c.expectData(2, 2*time.Second)

	c.ack(1)
	c.expectSilence(quietWindow, "DATA sent in answer to a duplicate ACK")

	c.ack(2)
	c.expectData(3, 2*time.Second)
	c.ack(3)
	c.expectSilence(quietWindow, "DATA sent after the final block was acknowledged")
}

// RFC requirement: RFC1350-4-3 positive -- the server chooses its own TID for the
// transfer: every DATA block of a three-block transfer comes from one source port, and
// that port is not the request port the RRQ was sent to.
func TestRFC1350ServerChoosesOneTIDForTheTransfer(t *testing.T) {
	t.Parallel()

	c := newTransferClient(t, map[string][]byte{"f.bin": patterned(1500)})
	c.sendRRQ("f.bin")

	var tid *net.UDPAddr
	for block := uint16(1); block <= 3; block++ {
		pkt, source, ok := c.read(2 * time.Second)
		if !ok {
			t.Fatalf("no DATA block %d", block)
		}
		requireData(t, pkt, block)
		if source.Port == c.listener.Port {
			t.Fatalf("block %d sent from the request port %d, not from a TID the server chose", block, source.Port)
		}
		if tid == nil {
			tid = source
		}
		if source.Port != tid.Port {
			t.Fatalf("block %d sent from %v, the transfer started on TID %v", block, source, tid)
		}
		c.ack(block)
	}
}

// RFC requirement: RFC1350-5-3 positive -- every data field is 0 to 512 octets (read into
// a 64 KiB buffer, so an oversize block is seen whole); a 512-octet block is never the
// last (block 2 follows, and a non-final block is never short); a 0 to 511 octet block
// ends the transfer: after its ACK nothing more arrives, for a 476-octet end (1500-octet
// file), a 0-octet end after full blocks (1024-octet file), and an empty file.
func TestRFC1350DataFieldLengthEndsTheTransfer(t *testing.T) {
	t.Parallel()

	for _, size := range []int{1500, 1024, 0} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()

			content := patterned(size)
			c := newTransferClient(t, map[string][]byte{"f.bin": content})
			c.sendRRQ("f.bin")

			var received []byte
			for block := uint16(1); ; block++ {
				data := c.expectData(block, 2*time.Second)
				if len(data) > 512 {
					t.Fatalf("block %d carries %d octets, more than 512", block, len(data))
				}
				received = append(received, data...)
				c.ack(block)
				if len(data) < 512 {
					break
				}
				if len(received) > size {
					t.Fatalf("a full block %d was sent past the end of a %d-octet file", block, size)
				}
			}

			if !bytes.Equal(received, content) {
				t.Fatalf("received %d octets, want the %d-octet file", len(received), size)
			}
			c.expectSilence(quietWindow, "DATA sent after a short block ended the transfer")
		})
	}
}

// RFC requirement: RFC1350-6-1 positive -- the last DATA, left unacknowledged, is
// retransmitted byte for byte after the ACK timeout.
// RFC requirement: RFC1350-6-1 negative -- once the last DATA is acknowledged it is not
// retransmitted: nothing arrives for longer than the ACK timeout after the ACK.
func TestRFC1350LastDATARetransmittedUntilAcknowledged(t *testing.T) {
	t.Parallel()

	c := newTransferClient(t, map[string][]byte{"f.bin": patterned(1000)})
	c.sendRRQ("f.bin")

	c.expectData(1, 2*time.Second)
	c.ack(1)

	last, _, ok := c.read(2 * time.Second)
	if !ok {
		t.Fatal("no DATA block 2")
	}
	requireData(t, last, 2)

	again, _, ok := c.read(ackTimeout + 2*time.Second)
	if !ok {
		t.Fatal("the unacknowledged last DATA was not retransmitted")
	}
	if !bytes.Equal(again, last) {
		t.Fatal("the retransmission differs from the last DATA")
	}

	c.ack(2)
	c.expectSilence(ackTimeout+2*time.Second, "the last DATA was retransmitted after its ACK")
}

// RFC requirement: RFC1350-7-1 positive -- the ACK timeout detects a peer that has gone:
// with no ACK ever sent the server sends block 1 exactly maxRetransmit+1 times, then
// stops, and the transfer slot is released, so a new RRQ on a one-slot server is served.
func TestRFC1350TimeoutEndsTheTransferOfAGonePeer(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "f.bin"), patterned(20), 0o644); err != nil {
		t.Fatal(err)
	}
	listener := startTestTFTPServer(t, rootDir, 1)

	gone := newTransferClientFor(t, listener)
	gone.sendRRQ("f.bin")

	copies := 0
	for {
		pkt, _, ok := gone.read(ackTimeout + 2*time.Second)
		if !ok {
			break
		}
		requireData(t, pkt, 1)
		copies++
		if copies > maxRetransmit+1 {
			t.Fatalf("block 1 sent %d times, the server never declared the peer gone", copies)
		}
	}
	if copies != maxRetransmit+1 {
		t.Fatalf("block 1 sent %d times, want %d before the transfer is abandoned", copies, maxRetransmit+1)
	}

	next := newTransferClientFor(t, listener)
	next.sendRRQ("f.bin")
	next.expectData(1, 2*time.Second)
}

// RFC requirement: RFC1350-7-1 negative -- a timeout followed by an answer is not an
// error: when the ACK of block 1 arrives after one retransmission, the transfer goes on
// and block 2 is sent.
func TestRFC1350TimeoutThenACKContinuesTheTransfer(t *testing.T) {
	t.Parallel()

	c := newTransferClient(t, map[string][]byte{"f.bin": patterned(1000)})
	c.sendRRQ("f.bin")

	c.expectData(1, 2*time.Second)
	c.expectData(1, ackTimeout+2*time.Second)

	c.ack(1)
	c.expectData(2, 2*time.Second)
	c.ack(2)
}
