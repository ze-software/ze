// VALIDATES: the octets BuildRRQ, WriteACK and BuildError put on the wire, and
// BuildRRQ's refusals. Method: each encoder against the hand-written packet.
// PREVENTS: a request with an option out of its RFC range, a field without its
// NUL, or a request over 512 octets reaching a server.
package tftp

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildRRQOctets(t *testing.T) {
	got, err := BuildRRQ("dir/f.bin", Request{Blksize: 1468, Tsize: true, Windowsize: 4})
	if err != nil {
		t.Fatalf("BuildRRQ: %v", err)
	}
	want := []byte("\x00\x01dir/f.bin\x00octet\x00blksize\x001468\x00tsize\x000\x00windowsize\x004\x00")
	if !bytes.Equal(got, want) {
		t.Fatalf("BuildRRQ = %q, want %q", got, want)
	}

	got, err = BuildRRQ("f", Request{})
	if err != nil {
		t.Fatalf("BuildRRQ with no option: %v", err)
	}
	if want := []byte("\x00\x01f\x00octet\x00"); !bytes.Equal(got, want) {
		t.Fatalf("BuildRRQ with no option = %q, want %q", got, want)
	}
}

func TestBuildRRQRefusals(t *testing.T) {
	refused := map[string]struct {
		file string
		req  Request
		want string
	}{
		"empty name":       {"", Request{}, "file name is empty"},
		"blksize 7":        {"f", Request{Blksize: 7}, "outside 8..65464"},
		"blksize 65465":    {"f", Request{Blksize: 65465}, "outside 8..65464"},
		"windowsize -1":    {"f", Request{Windowsize: -1}, "outside 1..65535"},
		"windowsize 65536": {"f", Request{Windowsize: 65536}, "outside 1..65535"},
		"too long":         {strings.Repeat("a", 500), Request{Blksize: 1468, Tsize: true}, "exceed 512 octets"},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			_, err := BuildRRQ(tc.file, tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("BuildRRQ(%q, %+v) = %v, want an error naming %q", tc.file, tc.req, err, tc.want)
			}
		})
	}

	// The inclusive bounds are accepted.
	for _, req := range []Request{{Blksize: 8, Windowsize: 1}, {Blksize: 65464, Windowsize: 65535}} {
		if _, err := BuildRRQ("f", req); err != nil {
			t.Fatalf("BuildRRQ at the bounds %+v: %v", req, err)
		}
	}

	// A request of exactly 512 octets is the largest a server accepts.
	file := strings.Repeat("a", RequestOctetsMax-len("\x00\x01\x00octet\x00"))
	pkt, err := BuildRRQ(file, Request{})
	if err != nil {
		t.Fatalf("BuildRRQ of a 512-octet request: %v", err)
	}
	if len(pkt) != RequestOctetsMax {
		t.Fatalf("request is %d octets, want %d", len(pkt), RequestOctetsMax)
	}
}

func TestWriteACKAndBuildError(t *testing.T) {
	var ack [HeaderOctets]byte
	if n := WriteACK(ack[:], 0xfffe); n != HeaderOctets {
		t.Fatalf("WriteACK wrote %d octets, want %d", n, HeaderOctets)
	}
	if want := []byte{0, 4, 0xff, 0xfe}; !bytes.Equal(ack[:], want) {
		t.Fatalf("WriteACK = %x, want %x", ack, want)
	}

	if got, want := BuildError(CodeOptionNegotiation, "bad"), []byte("\x00\x05\x00\x08bad\x00"); !bytes.Equal(got, want) {
		t.Fatalf("BuildError = %q, want %q", got, want)
	}
}
