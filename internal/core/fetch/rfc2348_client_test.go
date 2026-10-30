// VALIDATES: RFC 2348 (TFTP Blocksize Option) on the client side: the client
// uses the blocksize the OACK grants, refuses one above its request or below
// the minimum with ERROR 8, and ends the file on a block shorter than the
// negotiated size.
// PREVENTS: a client that writes a block larger than it asked for, or ends a
// transfer early or late because it judged the final block against the wrong
// size.
package fetch

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
)

// RFC requirement: RFC2348-x-2 positive -- an OACK granting blksize 1468 (the value
// requested) or 8 (the minimum) is used: the client confirms with ACK 0, takes a block of
// exactly that size as a full block, and the file holds every block.
// RFC requirement: RFC2348-x-2 negative -- an OACK granting blksize 1469 (above the 1468
// requested), 2000, or 7 (below the minimum) is answered with ERROR 8, no ACK, and the
// fetch fails.
// RFC requirement: RFC2348-x-3 positive -- a granted blksize of 8, the lowest valid value,
// and of 1468, inside 8..65464, is used.
// RFC requirement: RFC2348-x-3 negative -- a granted blksize of 7, below the valid range, is
// answered with ERROR 8 and the fetch fails.
func TestRFC2348ClientBlocksize(t *testing.T) {
	shortenTFTP(t)

	for _, size := range []int{tftp.BlksizeEthernet, tftp.BlksizeMin} {
		t.Run("granted "+strconv.Itoa(size), func(t *testing.T) {
			full, last := block(1, size), block(2, size-1)
			target, wait := startTFTPScript(t, func(p *tftpPeer) {
				p.oack(tftp.OptionBlksize, strconv.Itoa(size))
				if !p.expectACK(0) {
					return
				}
				p.data(1, full)
				if !p.expectACK(1) {
					return
				}
				p.data(2, last)
				p.expectACK(2)
			})
			got, err := fetchTFTP(t, target)
			wait()
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if !bytes.Equal(got, append(append([]byte(nil), full...), last...)) {
				t.Fatalf("fetched %d octets, want %d", len(got), 2*size-1)
			}
		})
	}

	for _, value := range []string{"1469", "2000", "7"} {
		t.Run("refused "+value, func(t *testing.T) {
			target, wait := startTFTPScript(t, func(p *tftpPeer) {
				p.oack(tftp.OptionBlksize, value)
				p.expectError(tftp.CodeOptionNegotiation)
				p.expectSilence(300*time.Millisecond, "the transfer ended with the ERROR")
			})
			_, err := fetchTFTP(t, target)
			wait()
			if err == nil {
				t.Fatalf("the fetch accepted blksize %s", value)
			}
		})
	}
}

// RFC requirement: RFC2348-x-4 positive -- with blksize 1024 negotiated, a 1023-octet
// block is the final packet: the client ACKs it, the fetch ends, and a block sent after it
// is not written.
// RFC requirement: RFC2348-x-4 negative -- a block of exactly 1024 octets is not the final
// packet: the client waits for the next block, so the file holds both.
func TestRFC2348ClientFinalBlockByNegotiatedSize(t *testing.T) {
	shortenTFTP(t)

	full, short := block(1, 1024), block(2, 1023)
	target, wait := startTFTPScript(t, func(p *tftpPeer) {
		p.oack(tftp.OptionBlksize, "1024")
		if !p.expectACK(0) {
			return
		}
		p.data(1, full)
		if !p.expectACK(1) {
			return
		}
		p.data(2, short)
		if !p.expectACK(2) {
			return
		}
		p.data(3, block(3, 50))
	})
	got, err := fetchTFTP(t, target)
	wait()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !bytes.Equal(got, append(append([]byte(nil), full...), short...)) {
		t.Fatalf("fetched %d octets, want 2047: the transfer did not end on the short block", len(got))
	}
}
