// VALIDATES: the RFC 2349 timeout option on Ze's TFTP server, observed from a
// loopback client: a requested timeout of 1 to 255 seconds is acknowledged
// with the client's own value and becomes the retransmission interval, and any
// other value is not acknowledged and leaves the configured interval.
// PREVENTS: an OACK that counter-proposes a timeout, a server that acknowledges
// a timeout it then ignores, and an out-of-range value (0 above all) used as the
// interval.
package tftpserver

import (
	"bytes"
	"testing"
	"time"
)

// RFC requirement: RFC2349-x-1 positive -- a requested timeout is acknowledged with the
// client's own value and used: timeout 1 and timeout 255 come back unchanged in the OACK,
// and with timeout 1 an unacknowledged DATA 1 is sent again within three seconds, well
// before the five-second default would fire.
// RFC requirement: RFC2349-x-1 negative -- the interval is the client's and no shorter: the
// resent DATA 1 comes no sooner than 0.8 s after the first one.
func TestRFC2349ServerAcknowledgesTimeout(t *testing.T) {
	t.Parallel()

	for _, seconds := range []string{"1", "255"} {
		t.Run("echo "+seconds, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"t.bin": patterned(100)})
			c.sendRRQ("t.bin", "timeout", seconds)
			opts, _ := c.expectOACK()
			if opts["timeout"] != seconds {
				t.Fatalf("OACK timeout = %q, want the requested %s (options %v)", opts["timeout"], seconds, opts)
			}
		})
	}

	t.Run("timeout 1 is the retransmission interval", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"t.bin": patterned(100)})
		c.sendRRQ("t.bin", "timeout", "1")
		c.expectOACK()
		c.ack(0)
		first := c.expectData(1, 2*time.Second)
		sent := time.Now()
		again := c.expectData(1, 3*time.Second)
		if elapsed := time.Since(sent); elapsed < 800*time.Millisecond {
			t.Fatalf("DATA 1 resent after %v, before the negotiated 1 s timeout", elapsed)
		}
		if !bytes.Equal(again, first) {
			t.Fatal("the resent DATA 1 carries other bytes than the first send")
		}
	})
}

// RFC requirement: RFC2349-x-2 positive -- a timeout in 1..255 is valid: beside tsize 0, the
// bounds 1 and 255 are acknowledged in the OACK.
// RFC requirement: RFC2349-x-2 negative -- a value outside 1..255, or one that is not an
// ASCII decimal number, is not a valid timeout: 0, 256, abc, +3 and the empty value, each
// beside tsize 0, draw an OACK carrying tsize and no timeout; with timeout 0 declined the
// server keeps its five-second default, so no DATA 1 is resent within two seconds; and an
// RRQ whose only option is timeout 0 is answered with DATA 1 and no OACK.
func TestRFC2349ServerDeclinesTimeoutOutOfRange(t *testing.T) {
	t.Parallel()

	for _, valid := range []string{"1", "255"} {
		t.Run("valid "+valid, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"v.bin": patterned(100)})
			c.sendRRQ("v.bin", "tsize", "0", "timeout", valid)
			opts, _ := c.expectOACK()
			if opts["timeout"] != valid {
				t.Fatalf("OACK timeout = %q, want %s (options %v)", opts["timeout"], valid, opts)
			}
		})
	}

	for _, invalid := range []string{"0", "256", "abc", "+3", ""} {
		t.Run("invalid "+invalid, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"i.bin": patterned(100)})
			c.sendRRQ("i.bin", "tsize", "0", "timeout", invalid)
			opts, _ := c.expectOACK()
			if value, ok := opts["timeout"]; ok {
				t.Fatalf("OACK acknowledges the invalid timeout %q as %q (options %v)", invalid, value, opts)
			}
			if opts["tsize"] != "100" {
				t.Fatalf("OACK tsize = %q, want 100 (options %v)", opts["tsize"], opts)
			}
			if invalid != "0" {
				return
			}
			c.ack(0)
			c.expectData(1, 2*time.Second)
			c.expectSilence(2*time.Second, "DATA 1 resent within 2 s: the declined timeout 0 was used")
		})
	}

	t.Run("invalid timeout alone", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"a.bin": patterned(100)})
		c.sendRRQ("a.bin", "timeout", "0")
		c.expectData(1, 2*time.Second)
	})
}
