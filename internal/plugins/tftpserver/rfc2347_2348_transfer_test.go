// VALIDATES: RFC 2347, RFC 2348 and RFC 7440 behavior past the OACK: which
// options the server acknowledges, that it uses only those, the inclusive
// blksize bounds, the end-of-transfer rules at a negotiated blocksize, and the
// NUL-terminated ASCII fields of a request and of the OACK.
// PREVENTS: a server that initiates negotiation, uses an option it declined,
// ends a transfer against 512 instead of the negotiated blocksize, or builds a
// field without its NUL.
package tftpserver

import (
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
	"time"
)

// expectOACK requires the next datagram to be an OACK made of whole key/value
// pairs and returns the options, keys lower-cased, and the raw datagram.
func (c *transferClient) expectOACK() (map[string]string, []byte) {
	c.t.Helper()
	pkt, _, ok := c.read(2 * time.Second)
	if !ok {
		c.t.Fatal("no OACK within 2 s")
	}
	if len(pkt) < 2 || binary.BigEndian.Uint16(pkt[0:2]) != opOACK {
		c.t.Fatalf("first reply is not an OACK: % x", pkt)
	}
	opts := map[string]string{}
	rest := pkt[2:]
	for len(rest) > 0 {
		key, afterKey, ok := nextCString(rest)
		if !ok {
			c.t.Fatalf("OACK option name without its NUL: % x", pkt)
		}
		value, afterValue, ok := nextCString(afterKey)
		if !ok {
			c.t.Fatalf("OACK option %q without a NUL-terminated value: % x", key, pkt)
		}
		opts[strings.ToLower(key)] = value
		rest = afterValue
	}
	return opts, pkt
}

// negotiateBlksize sends an RRQ for filename with blksize, requires the OACK to
// acknowledge acked, and confirms the OACK with ACK 0.
func negotiateBlksize(c *transferClient, filename, blksize, acked string) {
	c.t.Helper()
	c.sendRRQ(filename, "blksize", blksize)
	opts, _ := c.expectOACK()
	if opts["blksize"] != acked {
		c.t.Fatalf("OACK blksize = %q, want %q (options %v)", opts["blksize"], acked, opts)
	}
	c.ack(0)
}

// receiveBlockSizes reads the transfer to its end, ACKing each block, and
// returns the data length of each block. The end is the first block shorter
// than blocksize. It then requires that nothing more arrives.
func receiveBlockSizes(c *transferClient, blocksize int) []int {
	c.t.Helper()
	var sizes []int
	for block := uint16(1); ; block++ {
		data := c.expectData(block, 2*time.Second)
		sizes = append(sizes, len(data))
		c.ack(block)
		if len(data) < blocksize {
			break
		}
		if len(sizes) > 64 {
			c.t.Fatalf("no final block after %d blocks: %v", len(sizes), sizes)
		}
	}
	c.expectSilence(quietWindow, "DATA sent after the final packet")
	return sizes
}

// requireSizes fails unless got equals want.
func requireSizes(t *testing.T, got []int, want ...int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("block sizes %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("block sizes %v, want %v", got, want)
		}
	}
}

// RFC requirement: RFC2347-x-1 positive -- an option the client requested is acknowledged:
// an RRQ asking only for tsize gets an OACK whose tsize is the file length.
// RFC requirement: RFC2347-x-1 negative -- the server includes no option the client did not
// request and never starts negotiation itself: the tsize-only OACK carries no blksize, and
// an RRQ with no option, or with only the unsupported windowsize, is answered with DATA
// block 1 and no OACK.
func TestRFC2347OnlyTheClientInitiatesNegotiation(t *testing.T) {
	t.Parallel()

	t.Run("tsize only", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"t.bin": patterned(1500)})
		c.sendRRQ("t.bin", "tsize", "0")
		opts, _ := c.expectOACK()
		if opts["tsize"] != "1500" {
			t.Fatalf("OACK tsize = %q, want 1500 (options %v)", opts["tsize"], opts)
		}
		if _, ok := opts["blksize"]; ok {
			t.Fatalf("OACK carries blksize, which the client did not request: %v", opts)
		}
	})

	for name, opts := range map[string][]string{
		"no option":               nil,
		"unsupported option only": {"windowsize", "4"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"n.bin": patterned(100)})
			c.sendRRQ("n.bin", opts...)
			c.expectData(1, 2*time.Second)
		})
	}
}

// RFC requirement: RFC2347-x-3 positive -- an option the server did not acknowledge is
// ignored as if never requested: windowsize 4, asked beside blksize, is absent from the
// OACK and no second block arrives before ACK 1 (lockstep, not a window); a declined
// blksize (7 or 65465, asked beside tsize) is absent from the OACK and block 1 carries the
// default 512 octets.
// RFC requirement: RFC2347-x-3 negative -- an acknowledged option is used: the blksize 1024
// acknowledged beside windowsize makes block 1 carry 1024 octets.
func TestRFC2347UnacknowledgedOptionIsNotUsed(t *testing.T) {
	t.Parallel()

	t.Run("windowsize beside blksize", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"w.bin": patterned(3000)})
		c.sendRRQ("w.bin", "blksize", "1024", "windowsize", "4")
		opts, _ := c.expectOACK()
		if _, ok := opts["windowsize"]; ok {
			t.Fatalf("OACK acknowledges windowsize: %v", opts)
		}
		if opts["blksize"] != "1024" {
			t.Fatalf("OACK blksize = %q, want 1024 (options %v)", opts["blksize"], opts)
		}
		c.ack(0)
		if data := c.expectData(1, 2*time.Second); len(data) != 1024 {
			t.Fatalf("block 1 carries %d octets, want the acknowledged 1024", len(data))
		}
		c.expectSilence(quietWindow, "a second block sent before ACK 1: the unacknowledged windowsize was used")
		c.ack(1)
		c.expectData(2, 2*time.Second)
	})

	for _, declined := range []string{"7", "65465"} {
		t.Run("declined blksize "+declined, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"d.bin": patterned(1500)})
			c.sendRRQ("d.bin", "blksize", declined, "tsize", "0")
			opts, _ := c.expectOACK()
			if _, ok := opts["blksize"]; ok {
				t.Fatalf("OACK acknowledges the out-of-range blksize %s: %v", declined, opts)
			}
			c.ack(0)
			if data := c.expectData(1, 2*time.Second); len(data) != defaultBlockSize {
				t.Fatalf("block 1 carries %d octets, want the default %d", len(data), defaultBlockSize)
			}
		})
	}
}

// RFC requirement: RFC2348-x-3 positive -- both inclusive bounds are valid: blksize 8 is
// acknowledged as 8 and block 1 then carries 8 octets; blksize 65464 is acknowledged (as
// 1468, the server's Ethernet cap, which does not exceed the request).
// RFC requirement: RFC2348-x-3 negative -- one past each bound is invalid: blksize 7 and
// 65465, asked beside tsize, are absent from the OACK.
func TestRFC2348BlksizeBoundsAreInclusive(t *testing.T) {
	t.Parallel()

	t.Run("8", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"b.bin": patterned(20)})
		negotiateBlksize(c, "b.bin", "8", "8")
		if data := c.expectData(1, 2*time.Second); len(data) != 8 {
			t.Fatalf("block 1 carries %d octets, want 8", len(data))
		}
	})

	t.Run("65464", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"b.bin": patterned(20)})
		negotiateBlksize(c, "b.bin", "65464", strconv.Itoa(blksizeEthernet))
	})

	for _, outside := range []string{"7", "65465"} {
		t.Run(outside, func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"b.bin": patterned(20)})
			c.sendRRQ("b.bin", "blksize", outside, "tsize", "0")
			opts, _ := c.expectOACK()
			if _, ok := opts["blksize"]; ok {
				t.Fatalf("blksize %s is outside 8..65464 and was acknowledged: %v", outside, opts)
			}
		})
	}
}

// RFC requirement: RFC2348-x-4 positive -- at a negotiated blocksize of 1024, a block
// shorter than 1024 is the final packet: a 600-octet file is one 600-octet block and
// nothing follows its ACK (a server comparing against 512 would send an empty block 2),
// and a 2500-octet file ends on its 452-octet third block.
// RFC requirement: RFC2348-x-4 negative -- a block of exactly the negotiated 1024 octets is
// not the final packet: the 2500-octet file goes on to blocks 2 and 3.
func TestRFC2348ShortBlockIsTheFinalPacket(t *testing.T) {
	t.Parallel()

	t.Run("600", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"s.bin": patterned(600)})
		negotiateBlksize(c, "s.bin", "1024", "1024")
		requireSizes(t, receiveBlockSizes(c, 1024), 600)
	})

	t.Run("2500", func(t *testing.T) {
		t.Parallel()
		c := newTransferClient(t, map[string][]byte{"s.bin": patterned(2500)})
		negotiateBlksize(c, "s.bin", "1024", "1024")
		requireSizes(t, receiveBlockSizes(c, 1024), 1024, 1024, 452)
	})
}

// RFC requirement: RFC2348-x-5 positive -- a transfer that is an integral multiple of the
// negotiated blocksize 1024 ends with an extra DATA packet carrying no data: a 2048-octet
// file is sent as 1024, 1024, 0.
// RFC requirement: RFC2348-x-5 negative -- a transfer that is not a multiple gets no extra
// packet: a 1500-octet file is 1024, 476, and a 512-octet file (a multiple of 512, not of
// 1024) is one 512-octet block, with nothing after either.
func TestRFC2348ZeroLengthBlockEndsAnExactMultiple(t *testing.T) {
	t.Parallel()

	for size, want := range map[int][]int{
		2048: {1024, 1024, 0},
		1500: {1024, 476},
		512:  {512},
	} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()
			c := newTransferClient(t, map[string][]byte{"m.bin": patterned(size)})
			negotiateBlksize(c, "m.bin", "1024", "1024")
			requireSizes(t, receiveBlockSizes(c, 1024), want...)
		})
	}
}

// RFC requirement: RFC7440-3-1 positive -- the fields after the opcode are ASCII strings,
// each followed by one NUL: the Section 3 example request (foobar, octet, windowsize, 16)
// parses into its filename, mode and windowsize option, and every field of the OACK the
// server sends is a non-empty printable ASCII string followed by exactly one NUL.
// RFC requirement: RFC7440-3-1 negative -- a field not followed by its NUL is not taken as
// a field: a request whose #blocks value lacks its NUL leaves windowsize unrecognized, and
// one whose filename lacks its NUL is refused.
func TestRFC7440FieldsAreNULTerminatedASCIIStrings(t *testing.T) {
	t.Parallel()

	example := buildRRQPacket("foobar", "octet")
	example = append(example, "windowsize\x0016\x00"...)
	filename, mode, opts, err := parseRRQ(example)
	if err != nil {
		t.Fatalf("the Section 3 example request is refused: %v", err)
	}
	if filename != "foobar" || mode != "octet" || !opts.windowsize {
		t.Fatalf("parsed filename %q mode %q windowsize %v, want foobar octet true", filename, mode, opts.windowsize)
	}

	unterminated := buildRRQPacket("foobar", "octet")
	unterminated = append(unterminated, "windowsize\x0016"...)
	_, _, opts, err = parseRRQ(unterminated)
	if err != nil {
		t.Fatalf("request with an unterminated option value refused outright: %v", err)
	}
	if opts.windowsize {
		t.Fatal("windowsize recognized although its #blocks field has no NUL")
	}

	noFilenameNUL := binary.BigEndian.AppendUint16(nil, opRRQ)
	noFilenameNUL = append(noFilenameNUL, "foobar"...)
	if _, _, _, err := parseRRQ(noFilenameNUL); err == nil {
		t.Fatal("a request whose filename has no NUL was accepted")
	}

	c := newTransferClient(t, map[string][]byte{"a.bin": patterned(700)})
	c.sendRRQ("a.bin", "blksize", "1024", "tsize", "0", "windowsize", "16")
	_, pkt := c.expectOACK()
	fields := 0
	for field := range strings.SplitSeq(string(pkt[2:len(pkt)-1]), "\x00") {
		if field == "" {
			t.Fatalf("OACK carries an empty field or a doubled NUL: % x", pkt)
		}
		for i := range len(field) {
			if field[i] < 0x20 || field[i] > 0x7e {
				t.Fatalf("OACK field %q holds a non-ASCII octet 0x%02x", field, field[i])
			}
		}
		fields++
	}
	if pkt[len(pkt)-1] != 0 || fields != 4 {
		t.Fatalf("OACK is not four NUL-terminated fields (blksize, 1024, tsize, 700): % x", pkt)
	}
}
