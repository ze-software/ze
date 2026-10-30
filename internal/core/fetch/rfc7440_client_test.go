// VALIDATES: RFC 7440 (TFTP Windowsize Option) on the client, the data
// receiver of a read: it uses the windowsize the OACK grants, refuses one out
// of range or above its request, acknowledges only the last block of each
// window, falls back to RFC 1350 lockstep with a window of one block, and
// answers a gap with the ACK of the last block received in order.
// PREVENTS: a client that acknowledges every block of a window, never
// acknowledges a window, or accepts a window it never asked for.
package fetch

import (
	"bytes"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
)

// RFC requirement: RFC7440-3-4 positive -- an OACK granting windowsize 2 (the client asked
// for 4) is used: two blocks are sent before the client's ACK, and the file is complete.
// RFC requirement: RFC7440-3-2 positive -- windowsize 2, inside 1..65535 and not above the
// request, is accepted with ACK 0.
// RFC requirement: RFC7440-4-2 positive -- the client acknowledges the last block of each
// window: blocks 1 and 2 draw ACK 2, blocks 3 and 4 draw ACK 4, and the short block 5 draws
// ACK 5.
// RFC requirement: RFC7440-4-2 negative -- the first block of a window draws no ACK: after
// blocks 1 and 2 the client's first packet is ACK 2, not ACK 1.
// RFC requirement: RFC7440-3-4 negative -- an OACK granting windowsize 5, above the 4
// requested, is answered with ERROR 8 and the fetch fails.
// RFC requirement: RFC7440-3-2 negative -- an OACK granting windowsize 0 or 65536, outside
// 1..65535, is answered with ERROR 8 and the fetch fails.
func TestRFC7440ClientWindowsize(t *testing.T) {
	shortenTFTP(t)

	t.Run("window of two", func(t *testing.T) {
		blocks := [][]byte{block(1, 8), block(2, 8), block(3, 8), block(4, 8), block(5, 3)}
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			p.oack(tftp.OptionBlksize, "8", tftp.OptionWindowsize, "2")
			if !p.expectACK(0) {
				return
			}
			p.data(1, blocks[0])
			p.data(2, blocks[1])
			if !p.expectACK(2) {
				return
			}
			p.data(3, blocks[2])
			p.data(4, blocks[3])
			if !p.expectACK(4) {
				return
			}
			p.data(5, blocks[4])
			p.expectACK(5)
		})
		got, err := fetchTFTP(t, target)
		wait()
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if want := bytes.Join(blocks, nil); !bytes.Equal(got, want) {
			t.Fatalf("fetched %d octets, want %d", len(got), len(want))
		}
	})

	for _, value := range []string{"5", "0", "65536"} {
		t.Run("refused "+value, func(t *testing.T) {
			target, wait := startTFTPScript(t, func(p *tftpPeer) {
				p.oack(tftp.OptionWindowsize, value)
				p.expectError(tftp.CodeOptionNegotiation)
				p.expectSilence(300*time.Millisecond, "the transfer ended with the ERROR")
			})
			_, err := fetchTFTP(t, target)
			wait()
			if err == nil {
				t.Fatalf("the fetch accepted windowsize %s", value)
			}
		})
	}
}

// RFC requirement: RFC7440-4-3 positive -- with windowsize 1 acknowledged, the traffic is
// RFC 1350 lockstep: every DATA draws its own ACK before the next DATA is sent.
// RFC requirement: RFC1350-2-3 positive -- every DATA the client accepts in lockstep draws
// its ACK before the server sends the next one.
// RFC requirement: RFC7440-4-3 negative -- a window of one holds no block past the expected
// one, as in RFC 1350: block 3 sent while block 2 is expected draws no ACK and is not
// written, and the file holds each block once.
func TestRFC7440ClientWindowOneIsLockstep(t *testing.T) {
	shortenTFTP(t)

	blocks := [][]byte{block(1, 8), block(2, 8), block(3, 8), block(4, 2)}
	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.oack(tftp.OptionBlksize, "8", tftp.OptionWindowsize, "1")
		if !p.expectACK(0) {
			return
		}
		for i, payload := range blocks {
			if i == 1 {
				p.data(3, blocks[2])
				p.expectSilence(300*time.Millisecond, "a block past the expected one is not acknowledged in lockstep")
			}
			p.data(uint16(i+1), payload)
			if !p.expectACK(uint16(i + 1)) {
				return
			}
		}
	})
	got, err := fetchTFTP(t, target)
	wait()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if want := bytes.Join(blocks, nil); !bytes.Equal(got, want) {
		t.Fatalf("fetched %d octets, want %d", len(got), len(want))
	}
}

// RFC requirement: RFC7440-4-6 positive -- in a window of four, block 2 is lost: block 3
// draws the ACK of block 1, the last block received in order, and the server's restarted
// window from block 2 completes the file.
// RFC requirement: RFC7440-4-6 negative -- block 4, still past the gap, draws no second ACK:
// the client notifies the sequence error once per window.
func TestRFC7440ClientSequenceError(t *testing.T) {
	shortenTFTP(t)

	blocks := [][]byte{block(1, 8), block(2, 8), block(3, 8), block(4, 8), block(5, 1)}
	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.oack(tftp.OptionBlksize, "8", tftp.OptionWindowsize, "4")
		if !p.expectACK(0) {
			return
		}
		p.data(1, blocks[0])
		p.data(3, blocks[2])
		if !p.expectACK(1) {
			return
		}
		p.data(4, blocks[3])
		p.expectSilence(300*time.Millisecond, "the gap was already acknowledged")
		for i := 1; i < len(blocks); i++ {
			p.data(uint16(i+1), blocks[i])
		}
		p.expectACK(5)
	})
	got, err := fetchTFTP(t, target)
	wait()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if want := bytes.Join(blocks, nil); !bytes.Equal(got, want) {
		t.Fatalf("fetched %d octets, want %d", len(got), len(want))
	}
}

// RFC requirement: RFC7440-3-1 positive -- every field of the client's read request after
// the opcode is an ASCII string followed by one NUL: the request is exactly
// "f.bin\0octet\0blksize\0" "1468\0tsize\0" "0\0windowsize\0" "4\0" after the opcode.
// RFC requirement: RFC7440-3-1 negative -- a file name holding a NUL byte or a byte outside
// ASCII is refused before any packet is sent.
func TestRFC7440ClientRequestFieldsAreNULTerminatedASCII(t *testing.T) {
	shortenTFTP(t)

	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		want := []byte("\x00\x01f.bin\x00octet\x00blksize\x001468\x00tsize\x000\x00windowsize\x004\x00")
		if !bytes.Equal(p.rrq, want) {
			p.t.Errorf("read request %q, want %q", p.rrq, want)
		}
		p.send(tftp.BuildError(tftp.CodeFileNotFound, "no"))
	})
	if _, err := fetchTFTP(t, target); err == nil {
		t.Fatal("the fetch ignored the server's ERROR")
	}
	wait()

	for _, name := range []string{"a\x00b", "caf\xc3\xa9"} {
		if _, err := tftp.BuildRRQ(name, tftpRequest); err == nil {
			t.Errorf("BuildRRQ(%q) built a request", name)
		}
	}
}
