// VALIDATES: RFC 2347 (TFTP Option Extension) on the client side: the client
// uses exactly the options the server acknowledged, and refuses an OACK that
// carries an option it never requested.
// PREVENTS: a client that keeps a requested blksize or windowsize the server
// never granted, which truncates the file or stalls the transfer, or one that
// accepts an option the server invented.
package fetch

import (
	"bytes"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
)

// RFC requirement: RFC2347-x-4 positive -- the client asked for blksize 1468, tsize and
// windowsize 4, and the OACK acknowledges only blksize 1024: the client confirms with ACK 0,
// takes 1024-octet blocks (a 1024-octet DATA is not final), and acknowledges each block at
// once, so it uses a window of one block rather than the windowsize it requested.
// RFC requirement: RFC2347-x-3 positive -- the windowsize the server did not acknowledge is
// ignored as if never requested: the client ACKs block 1 before block 2 is sent.
// RFC requirement: RFC2347-x-4 negative -- the server answers the RRQ with DATA 1 of 512
// octets, acknowledging no option: the client does not use its requested blksize of 1468,
// so it reads the 512-octet block as a full block, not the final one, ACKs it and takes the
// next blocks until a block shorter than 512 octets ends the file.
// RFC requirement: RFC2347-x-3 negative -- the same unacknowledged blksize is not used: the
// fetched file holds all three blocks, 1034 octets.
func TestRFC2347ClientUsesOnlyAcknowledgedOptions(t *testing.T) {
	shortenTFTP(t)

	t.Run("blksize acknowledged alone", func(t *testing.T) {
		first, last := block(1, 1024), block(2, 100)
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			p.oack(tftp.OptionBlksize, "1024")
			if !p.expectACK(0) {
				return
			}
			p.data(1, first)
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
		if !bytes.Equal(got, append(append([]byte(nil), first...), last...)) {
			t.Fatalf("fetched %d octets, want the 1124 octets of both blocks", len(got))
		}
	})

	t.Run("no option acknowledged", func(t *testing.T) {
		blocks := [][]byte{block(1, 512), block(2, 512), block(3, 10)}
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			for i, payload := range blocks {
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
			t.Fatalf("fetched %d octets, want %d: a 512-octet block was read as final", len(got), len(want))
		}
	})
}

// RFC requirement: RFC2347-x-6 positive -- an OACK carrying an option the client never
// requested ("timeout"), or naming a requested option twice, is answered with ERROR 8 and
// the transfer fails without any ACK.
// RFC requirement: RFC2347-x-6 negative -- an OACK carrying only requested options, in upper
// case, draws ACK 0 and no ERROR.
func TestRFC2347ClientRejectsUnrequestedOption(t *testing.T) {
	shortenTFTP(t)

	refused := map[string][]string{
		"unrequested": {tftp.OptionBlksize, "1024", "timeout", "5"},
		"twice":       {tftp.OptionBlksize, "1024", "BLKSIZE", "512"},
	}
	for name, oack := range refused {
		t.Run(name, func(t *testing.T) {
			target, wait := startTFTPScript(t, func(p *tftpPeer) {
				p.oack(oack...)
				p.expectError(tftp.CodeOptionNegotiation)
				p.expectSilence(300*time.Millisecond, "the transfer ended with the ERROR")
			})
			_, err := fetchTFTP(t, target)
			wait()
			if err == nil {
				t.Fatalf("the fetch accepted the OACK %q", oack)
			}
		})
	}

	t.Run("requested only", func(t *testing.T) {
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			p.oack("BLKSIZE", "1024", "TSize", "3")
			if !p.expectACK(0) {
				return
			}
			p.data(1, []byte("abc"))
			p.expectACK(1)
		})
		got, err := fetchTFTP(t, target)
		wait()
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		if string(got) != "abc" {
			t.Fatalf("fetched %q, want abc", got)
		}
	})
}
