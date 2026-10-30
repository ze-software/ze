// VALIDATES: RFC 2349 (TFTP Transfer Size Option) on the client side: the
// read request asks for the size with "0", and a size the client cannot hold
// is refused with ERROR 3 before any DATA.
// PREVENTS: a client that sends a made-up size in a read request, or that
// starts a transfer it can only abort at the 1 GiB bound.
package fetch

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
)

// RFC requirement: RFC2349-x-3 positive -- the client's read request carries the tsize
// option with the value "0" (the octets after the mode are blksize 1468, tsize 0,
// windowsize 4), and the size the OACK returns is taken: the transfer completes.
// RFC requirement: RFC2349-x-3 negative -- a tsize value in the OACK that is not a decimal
// number ("12k") is refused with ERROR 8 and the fetch fails.
func TestRFC2349ClientTsize(t *testing.T) {
	shortenTFTP(t)

	t.Run("size returned", func(t *testing.T) {
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			want := []byte("\x00\x01f.bin\x00octet\x00blksize\x001468\x00tsize\x000\x00windowsize\x004\x00")
			if !bytes.Equal(p.rrq, want) {
				p.t.Errorf("read request %q, want %q", p.rrq, want)
			}
			p.oack(tftp.OptionTsize, "3")
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

	t.Run("size not a number", func(t *testing.T) {
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			p.oack(tftp.OptionTsize, "12k")
			p.expectError(tftp.CodeOptionNegotiation)
		})
		_, err := fetchTFTP(t, target)
		wait()
		if err == nil {
			t.Fatal("the fetch accepted tsize 12k")
		}
	})
}

// RFC requirement: RFC2349-x-5 positive -- an OACK announcing a file one octet larger than
// the 1 GiB a fetch accepts is answered with ERROR 3 and no ACK 0, before any DATA.
// RFC requirement: RFC2349-x-5 negative -- an OACK announcing exactly 1 GiB is accepted with
// ACK 0.
func TestRFC2349ClientAbortsTooLargeFile(t *testing.T) {
	shortenTFTP(t)

	t.Run("too large", func(t *testing.T) {
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			p.oack(tftp.OptionTsize, strconv.FormatInt(fileSizeMax+1, 10))
			p.expectError(tftp.CodeDiskFull)
			p.expectSilence(300*time.Millisecond, "the transfer ended with the ERROR")
		})
		_, err := fetchTFTP(t, target)
		wait()
		if err == nil {
			t.Fatal("the fetch accepted a file above 1 GiB")
		}
	})

	t.Run("at the bound", func(t *testing.T) {
		target, wait := startTFTPScript(t, func(p *tftpPeer) {
			p.oack(tftp.OptionTsize, strconv.FormatInt(fileSizeMax, 10))
			if !p.expectACK(0) {
				return
			}
			p.data(1, nil)
			p.expectACK(1)
		})
		if _, err := fetchTFTP(t, target); err != nil {
			t.Fatalf("fetch: %v", err)
		}
		wait()
	})
}
