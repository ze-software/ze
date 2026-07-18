// VALIDATES: RFC 2348 (TFTP Blocksize option) server side -- the acknowledged
// blksize never exceeds the client's request and is confined to the valid range
// 8..65464; out-of-range requests are ignored.
// PREVENTS: a server that acknowledges a larger blocksize than asked (oversized
// datagrams) or accepts an out-of-range value.
package tftpserver

import (
	"strconv"
	"testing"
)

// RFC requirement: RFC2348-x-1 positive -- the server's acknowledged blksize is <=
// the client's requested value: a request for 1200 (within Ze's cap) is acknowledged
// as 1200 (RFC 2348; producer handleRRQ negotiatedBlksize = min(opts.blksize,
// blksizeEthernet), internal/plugins/tftpserver/handler.go:271-272).
// RFC requirement: RFC2348-x-1 negative -- the acknowledged blksize NEVER exceeds the
// request: a request for 60000 (above Ze's 1468 cap) is acknowledged as 1468, still
// <= 60000, so Ze never returns a larger blocksize than asked.
func TestRFC2348BlksizeAckNotAboveRequest(t *testing.T) {
	t.Parallel()

	t.Run("within cap acked as requested", func(t *testing.T) {
		t.Parallel()
		srv, client := tftpServeFile(t, "a.bin", []byte("data"))
		if _, err := client.WriteToUDP(buildRRQPacketWithOptions("a.bin", "blksize", "1200"), srv); err != nil {
			t.Fatal(err)
		}
		opts := readServerOACKOptions(t, client)
		if opts["blksize"] != "1200" {
			t.Errorf("blksize = %q, want 1200 (<= requested)", opts["blksize"])
		}
	})

	t.Run("above cap acked below the request", func(t *testing.T) {
		t.Parallel()
		srv, client := tftpServeFile(t, "b.bin", []byte("data"))
		if _, err := client.WriteToUDP(buildRRQPacketWithOptions("b.bin", "blksize", "60000"), srv); err != nil {
			t.Fatal(err)
		}
		opts := readServerOACKOptions(t, client)
		acked, err := strconv.Atoi(opts["blksize"])
		if err != nil {
			t.Fatalf("acked blksize %q is not numeric: %v", opts["blksize"], err)
		}
		if acked > 60000 {
			t.Errorf("acked blksize %d exceeds the requested 60000", acked)
		}
	})
}

// RFC requirement: RFC2348-x-3 positive -- a blksize within the valid range 8..65464
// is accepted and acknowledged: 512 is acknowledged in the OACK (RFC 2348; producer
// parseRRQ n >= blksizeMin && n <= blksizeMax, handler.go:122-125).
// RFC requirement: RFC2348-x-3 negative -- a blksize OUTSIDE 8..65464 is rejected: a
// too-small (5) or too-large (70000) value is ignored and does NOT appear in the OACK
// (the tsize option, requested alongside it, is still acknowledged), so the transfer
// falls back to the default blocksize.
func TestRFC2348BlksizeRangeEnforced(t *testing.T) {
	t.Parallel()

	t.Run("in-range 512 acked", func(t *testing.T) {
		t.Parallel()
		srv, client := tftpServeFile(t, "a.bin", []byte("data"))
		if _, err := client.WriteToUDP(buildRRQPacketWithOptions("a.bin", "blksize", "512"), srv); err != nil {
			t.Fatal(err)
		}
		opts := readServerOACKOptions(t, client)
		if opts["blksize"] != "512" {
			t.Errorf("in-range blksize 512 not acknowledged: %v", opts)
		}
	})

	for _, bad := range []string{"5", "70000"} {
		t.Run("out-of-range "+bad+" ignored", func(t *testing.T) {
			t.Parallel()
			srv, client := tftpServeFile(t, "c.bin", []byte("data"))
			// Request the out-of-range blksize alongside a valid tsize so an OACK is
			// still sent (blksize alone, if rejected, would produce no OACK at all).
			if _, err := client.WriteToUDP(buildRRQPacketWithOptions("c.bin", "blksize", bad, "tsize", "0"), srv); err != nil {
				t.Fatal(err)
			}
			opts := readServerOACKOptions(t, client)
			if _, ok := opts["blksize"]; ok {
				t.Errorf("out-of-range blksize %s was acknowledged: %v", bad, opts)
			}
			if _, ok := opts["tsize"]; !ok {
				t.Errorf("tsize not acknowledged (an OACK should still be sent): %v", opts)
			}
		})
	}
}
