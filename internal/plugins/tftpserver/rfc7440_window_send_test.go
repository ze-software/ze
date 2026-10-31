// VALIDATES: Ze's TFTP server as the RFC 7440 data sender, observed from a
// loopback client: the windowsize it acknowledges, the window it sends before
// it waits, the window it sends again after a timeout or an ACK inside the
// window, the lockstep a window of one is, and the limits the config sets.
// PREVENTS: a server that grants more than asked, streams past the window,
// restarts a window from the wrong block, answers a duplicate or stale ACK
// with DATA, or loses the block count where block numbers wrap at 65535.
package tftpserver

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/tftp"
)

// expectWindow requires DATA blocks from..from+count-1, in order, and returns
// their data fields.
func (c *transferClient) expectWindow(from uint16, count int) [][]byte {
	c.t.Helper()
	window := make([][]byte, 0, count)
	for i := range count {
		window = append(window, c.expectData(from+uint16(i), 2*time.Second)) //nolint:gosec // G115: a test window is far below 65536 blocks
	}
	return window
}

// startLimitedServer starts a server over one file under limits and returns
// a client for it.
func startLimitedServer(t *testing.T, name string, content []byte, limits transferLimits) *transferClient {
	t.Helper()
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, name), content, 0o644); err != nil {
		t.Fatal(err)
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeUDP(t, conn) })
	var source atomic.Pointer[transferLimits]
	source.Store(&limits)
	go serveLimited(newTransferGeneration(), conn, rootDir, make(chan struct{}, 4), &source, slogutil.DiscardLogger())
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("a UDP socket without a UDP address")
	}
	return newTransferClientFor(t, addr)
}

// RFC requirement: RFC7440-3-3 positive -- a requested windowsize is acknowledged in the OACK
// at a value no larger than requested: 8 comes back as 8, and 64, the configured maximum, as
// 64.
// RFC requirement: RFC7440-3-3 negative -- the server never acknowledges more than the client
// asked for nor more than its maximum: 1000 and 65535 come back as 64, never as the request,
// and 0, 65536 and abc are not acknowledged at all.
func TestRFC7440ServerAcknowledgesWindowsize(t *testing.T) {
	t.Parallel()

	for requested, want := range map[string]string{"8": "8", "64": "64", "1000": "64", "65535": "64"} {
		t.Run("request "+requested, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"w.bin": patterned(100)})
			c.sendRRQ("w.bin", "windowsize", requested)
			opts, _ := c.expectOACK()
			if opts["windowsize"] != want {
				t.Fatalf("OACK windowsize = %q for a request of %s, want %s (options %v)", opts["windowsize"], requested, want, opts)
			}
			granted, err := strconv.Atoi(opts["windowsize"])
			if err != nil {
				t.Fatalf("OACK windowsize %q is not a number", opts["windowsize"])
			}
			asked, err := strconv.Atoi(requested)
			if err != nil {
				t.Fatal(err)
			}
			if granted > asked {
				t.Fatalf("OACK windowsize %d is above the %d requested", granted, asked)
			}
		})
	}

	for _, invalid := range []string{"0", "65536", "abc"} {
		t.Run("invalid "+invalid, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"w.bin": patterned(100)})
			c.sendRRQ("w.bin", "blksize", "512", "windowsize", invalid)
			opts, _ := c.expectOACK()
			if value, ok := opts["windowsize"]; ok {
				t.Fatalf("OACK acknowledges the invalid windowsize %q as %q", invalid, value)
			}
		})
	}
}

// RFC requirement: RFC7440-4-1 positive -- with windowsize 4 agreed, the server sends four
// consecutive DATA blocks, then the next four once the window's last block is acknowledged,
// and the final short block alone as the last window.
// RFC requirement: RFC7440-4-1 negative -- the server stops after the window and waits: no
// fifth block arrives before ACK 4, and no block after the final one's ACK.
func TestRFC7440ServerSendsWindow(t *testing.T) {
	t.Parallel()

	content := patterned(8*tftp.DefaultBlockSize + 100)
	c := newTransferClient(t, map[string][]byte{"w.bin": content})
	c.sendRRQ("w.bin", "windowsize", "4")
	opts, _ := c.expectOACK()
	if opts["windowsize"] != "4" {
		t.Fatalf("OACK windowsize = %q, want 4", opts["windowsize"])
	}
	c.ack(0)
	got := c.expectWindow(1, 4)
	c.expectSilence(quietWindow, "a fifth block sent before the ACK of the window")
	c.ack(4)
	got = append(got, c.expectWindow(5, 4)...)
	c.expectSilence(quietWindow, "a ninth block sent before the ACK of the second window")
	c.ack(8)
	final := c.expectData(9, 2*time.Second)
	got = append(got, final)
	c.ack(9)
	c.expectSilence(quietWindow, "DATA after the final block was acknowledged")

	if !bytes.Equal(bytes.Join(got, nil), content) {
		t.Fatal("the windowed transfer delivered other bytes than the file")
	}
}

// RFC requirement: RFC7440-4-3 positive -- a window of one block is RFC 1350: a request for
// windowsize 1 alone is answered with DATA 1 and no OACK, and beside blksize it is left out
// of the OACK; block 2 follows ACK 1.
// RFC requirement: RFC7440-4-3 negative -- with a window of one block no second block is sent
// before ACK 1.
func TestRFC7440ServerWindowOneIsLockstep(t *testing.T) {
	t.Parallel()

	t.Run("windowsize 1 alone", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"l.bin": patterned(3 * tftp.DefaultBlockSize)})
		c.sendRRQ("l.bin", "windowsize", "1")
		c.expectData(1, 2*time.Second)
		c.expectSilence(quietWindow, "block 2 sent before ACK 1 with a window of one block")
		c.ack(1)
		c.expectData(2, 2*time.Second)
	})

	t.Run("windowsize 1 beside blksize", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"l.bin": patterned(3000)})
		c.sendRRQ("l.bin", "blksize", "1024", "windowsize", "1")
		opts, _ := c.expectOACK()
		if value, ok := opts["windowsize"]; ok {
			t.Fatalf("OACK acknowledges windowsize %q for a window of one block", value)
		}
		c.ack(0)
		c.expectData(1, 2*time.Second)
		c.expectSilence(quietWindow, "block 2 sent before ACK 1 with a window of one block")
		c.ack(1)
		c.expectData(2, 2*time.Second)
	})
}

// RFC requirement: RFC7440-4-4 positive -- when the ACK of a window does not come before the
// timeout, the server sends the window again from the block after the last ACK it received:
// with windowsize 4 and timeout 1, ACK 4 received and the second window not acknowledged,
// blocks 5 to 8 come again with the same bytes.
// RFC requirement: RFC7440-4-4 negative -- the restart neither goes back past the last ACK
// nor skips ahead: the window after the timeout starts at block 5, not 1 and not 9; and a
// window never acknowledged is sent four times in all, then the transfer ends.
func TestRFC7440ServerTimeoutRestartsWindow(t *testing.T) {
	t.Parallel()

	t.Run("restart from the last ACK", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"r.bin": patterned(12*tftp.DefaultBlockSize + 7)})
		c.sendRRQ("r.bin", "windowsize", "4", "timeout", "1")
		c.expectOACK()
		c.ack(0)
		c.expectWindow(1, 4)
		c.ack(4)
		first := c.expectWindow(5, 4)
		again := c.expectWindow(5, 4)
		for i := range first {
			if !bytes.Equal(first[i], again[i]) {
				t.Fatalf("block %d sent again with other bytes", 5+i)
			}
		}
		c.ack(8)
		c.expectWindow(9, 4)
	})

	t.Run("bounded", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"b.bin": patterned(8 * tftp.DefaultBlockSize)})
		c.sendRRQ("b.bin", "windowsize", "4", "timeout", "1")
		c.expectOACK()
		c.ack(0)
		for range maxRetransmit + 1 {
			c.expectWindow(1, 4)
		}
		c.expectSilence(2*time.Second, "the unacknowledged window sent more than four times")
	})
}

// RFC requirement: RFC7440-4-5 positive -- an ACK inside the window sets the start of the next
// one: with windowsize 4 and blocks 1 to 4 sent, ACK 2 is answered at once, before the
// five-second timeout, with the full window 3 to 6.
// RFC requirement: RFC7440-4-5 negative -- an ACK outside the window is not taken as one: ACK
// 2 again (the block before the window 3 to 6) and ACK 20 (a block never sent) draw no DATA
// before the timeout.
func TestRFC7440ServerInWindowACKRestartsWindow(t *testing.T) {
	t.Parallel()

	c := newTransferClient(t, map[string][]byte{"s.bin": patterned(12 * tftp.DefaultBlockSize)})
	c.sendRRQ("s.bin", "windowsize", "4")
	c.expectOACK()
	c.ack(0)
	c.expectWindow(1, 4)
	c.ack(2)
	start := time.Now()
	c.expectWindow(3, 4)
	if elapsed := time.Since(start); elapsed >= ackTimeout-time.Second {
		t.Fatalf("the window after ACK 2 came after %v, the timeout and not the ACK", elapsed)
	}
	c.ack(2)
	c.expectSilence(quietWindow, "DATA in answer to the ACK of the block before the window")
	c.ack(20)
	c.expectSilence(quietWindow, "DATA in answer to the ACK of a block never sent")
	c.ack(6)
	c.expectWindow(7, 4)
}

// TestTFTPWindowedTransferEdges drives whole windowed transfers to their end:
// a file of exactly three blocks (an empty fourth block ends it, inside the
// first window), an empty file, and a file of 65540 eight-octet blocks whose
// block numbers wrap past 65535 to 0.
// VALIDATES: AC-13, the end of a windowed transfer and the 16-bit wrap.
// PREVENTS: a window that stops a block early on an exact multiple, and ACKs
// after the wrap compared as plain integers.
func TestTFTPWindowedTransferEdges(t *testing.T) {
	t.Parallel()

	for name, size := range map[string]int{"exact": 3 * tftp.DefaultBlockSize, "empty": 0} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			content := patterned(size)
			c := newTransferClient(t, map[string][]byte{"e.bin": content})
			c.sendRRQ("e.bin", "windowsize", "4")
			c.expectOACK()
			c.ack(0)
			blocks := size/tftp.DefaultBlockSize + 1
			window := c.expectWindow(1, blocks)
			if len(window[blocks-1]) != 0 {
				t.Fatalf("the last block carries %d octets, want an empty block", len(window[blocks-1]))
			}
			c.ack(uint16(blocks)) //nolint:gosec // G115: at most 4
			c.expectSilence(quietWindow, "DATA after the final block was acknowledged")
			if !bytes.Equal(bytes.Join(window, nil), content) {
				t.Fatal("the transfer delivered other bytes than the file")
			}
		})
	}

	t.Run("wrap", func(t *testing.T) {
		t.Parallel()
		const blksize = 8
		content := patterned(65540*blksize + 3)
		c := newTransferClient(t, map[string][]byte{"big.bin": content})
		c.sendRRQ("big.bin", "blksize", "8", "windowsize", "64")
		opts, _ := c.expectOACK()
		if opts["windowsize"] != "64" {
			t.Fatalf("OACK windowsize = %q, want 64", opts["windowsize"])
		}
		c.ack(0)
		var got bytes.Buffer
		next := uint16(1) // wraps from 65535 to 0, as the wire does
		for final := false; !final; {
			var read uint16
			for range 64 {
				data := c.expectData(next, 2*time.Second)
				got.Write(data)
				read = next
				next++
				if len(data) < blksize {
					final = true
					break
				}
			}
			c.ack(read)
		}
		if !bytes.Equal(got.Bytes(), content) {
			t.Fatalf("delivered %d octets across the wrap, want the %d of the file", got.Len(), len(content))
		}
	})
}

// TestServerUsesConfiguredWindowAndTimeout serves under limits a config sets
// rather than the defaults: a window maximum of 2 and a timeout of 1 second.
// VALIDATES: AC-21, the configured max-windowsize caps the grant and the
// configured timeout is the interval when the client asks none.
// PREVENTS: limits read once at build and never from the running config.
func TestServerUsesConfiguredWindowAndTimeout(t *testing.T) {
	t.Parallel()

	c := startLimitedServer(t, "c.bin", patterned(4*tftp.DefaultBlockSize), transferLimits{timeout: time.Second, windowsizeMax: 2})
	c.sendRRQ("c.bin", "windowsize", "4")
	opts, _ := c.expectOACK()
	if opts["windowsize"] != "2" {
		t.Fatalf("OACK windowsize = %q, want the configured maximum 2", opts["windowsize"])
	}
	c.ack(0)
	c.expectWindow(1, 2)
	c.expectSilence(500*time.Millisecond, "a third block sent past the configured window of 2")
	c.expectWindow(1, 2) // the configured 1 s timeout, not the 5 s default
}

// TestWindowPositionAcrossTheWrap judges ACKs against windows that start
// before, at and after block number 65535.
// VALIDATES: the 16-bit position windowPosition answers across the wrap.
// PREVENTS: an ACK after the wrap read as a block before the window.
func TestWindowPositionAcrossTheWrap(t *testing.T) {
	t.Parallel()

	ack := func(block uint16) []byte {
		return binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, tftp.OpACK), block)
	}
	cases := []struct {
		first int64
		block uint16
		want  uint16
	}{
		{first: 1, block: 0, want: 0},
		{first: 1, block: 4, want: 4},
		{first: 65535, block: 65534, want: 0},
		{first: 65535, block: 65535, want: 1},
		{first: 65535, block: 0, want: 2},
		{first: 65535, block: 1, want: 3},
	}
	for _, tc := range cases {
		got, ok := windowPosition(ack(tc.block), tc.first)
		if !ok {
			t.Fatalf("ACK %d not read as an ACK", tc.block)
		}
		if got != tc.want {
			t.Fatalf("ACK %d against a window from %d: position %d, want %d", tc.block, tc.first, got, tc.want)
		}
	}
	if _, ok := windowPosition([]byte{0, byte(tftp.OpDATA), 0, 1}, 1); ok {
		t.Fatal("a DATA packet read as an ACK")
	}
}
