// Design: docs/architecture/isis/isis-6-lsdb.md -- own-LSP TLV encoding tests.
//
// VALIDATES: the TLV 137 (Dynamic Hostname) value Ze puts on the wire. The
// configured name reaches the LSP byte for byte. No sanitizer, no truncation, no
// ToASCII rewrite and no NUL terminator touches it. ISISHostnameValidator
// produces the character-set guarantee itself, at the config boundary
// (internal/component/config/validators.go). This file proves only that the emit
// path preserves what that boundary accepted.

package lsdb

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
)

// hostnameAccepted is a COPY of ISISHostnameAccepted
// (internal/component/config/validators_isis_test.go), kept identical by hand.
// It cannot be a shared variable: the original lives in a _test.go file, so no
// other package can import it, and lifting it into non-test code would put a
// test fixture in the shipped binary. So the copy CAN drift, and nothing in the
// tree fails when it does. Adding a value here that the config boundary refuses
// makes this file prove the emit path for a name no operator can configure.
var hostnameAccepted = []string{
	"r1",
	"r1-isis",
	"ze-p2p",
	"router-1.example.net",
	"router-1.example.net.",
	"core_1",
	"a router with spaces",
	" ",
	"~",
	strings.Repeat("a", 63),
	strings.Repeat("c.", 127) + "d", // 255 octets, the largest a config can carry
}

// emitHostnameTLV originates the node's own fragment 0 with the given hostname
// and returns the raw LSP bytes together with the decoded TLV 137 value. It
// returns a nil value when the LSP carries no TLV 137.
func emitHostnameTLV(t *testing.T, name string) (raw, value []byte) {
	t.Helper()
	d := New(nil)
	o := NewOriginator(d, nil)
	node := sampleNode(t)
	node.Hostname = name

	if res := o.Originate(Level2, node, LevelState{}); len(res.Originated) == 0 {
		t.Fatal("origination produced no LSP")
	}
	lsp := decodeOwnFrag0(t, d, Level2, node.SystemID)
	for _, tl := range lsp.TLVs {
		if tl.Type == packet.TLVDynamicHostname {
			value = tl.Value
			break
		}
	}
	e := d.Lookup(Level2, lsp.LSPID)
	if e == nil {
		t.Fatal("own fragment 0 vanished from the LSDB")
	}
	return e.Raw(), value
}

// TestISISHostnameTLVIsPrintableASCII: every value the config boundary accepts
// reaches the wire byte for byte, and every octet of it is printable 7-bit
// ASCII.
//
// RFC requirement: RFC5301-3-10 negative -- "If a user-interface for configuring
// or displaying this field permits Unicode characters, that user-interface is
// responsible for applying the ToASCII and/or ToUnicode algorithm as described
// in [RFC3490] to achieve the correct format for transmission or display"
// (RFC 5301 Section 3). Ze's user-interface refuses Unicode, so it owes no
// conversion, and it performs none: the configured octets are the emitted
// octets. A ToASCII rewrite on this path would change the operator's value.
//
// RFC requirement: RFC5301-3-8 negative -- "The string is not null-terminated"
// (RFC 5301 Section 3). No accepted value can carry a NUL octet, so no octet
// inside the framed value can be read as a terminator.
func TestISISHostnameTLVIsPrintableASCII(t *testing.T) {
	for _, name := range hostnameAccepted {
		raw, value := emitHostnameTLV(t, name)
		if value == nil {
			t.Errorf("hostname %q produced no TLV 137", name)
			continue
		}
		if string(value) != name {
			t.Errorf("TLV 137 value = %q, want %q (the emit path rewrote the operator's value)", value, name)
		}
		for i, c := range value {
			if c < 0x20 || c > 0x7e {
				t.Errorf("hostname %q: TLV 137 octet %d is 0x%02x, outside printable 7-bit ASCII", name, i+1, c)
			}
		}
		// The framing carries the value's own length, so the value is exactly
		// what the operator configured and nothing follows it inside the TLV.
		framed := append([]byte{packet.TLVDynamicHostname, byte(len(value))}, value...)
		if n := bytes.Count(raw, framed); n != 1 {
			t.Errorf("hostname %q: found %d framed TLV 137 runs in the raw LSP, want exactly 1", name, n)
		}
	}
}

// TestISISHostnameTLVFraming: TLV 137 carries type 137, a length octet equal to
// the value length, a value of 1 to 255 octets, and no NUL terminator.
//
// RFC requirement: RFC5301-3-4 positive -- "The Dynamic hostname TLV is defined
// here as TLV type 137" (RFC 5301 Section 3). The originated LSP frames the
// hostname under type 137.
//
// RFC requirement: RFC5301-3-5 positive -- "Length - total length of the value
// field" (RFC 5301 Section 3). The length octet equals the number of value
// octets that follow it.
//
// RFC requirement: RFC5301-3-6 positive -- "Value - a string of 1 to 255 bytes"
// (RFC 5301 Section 3). The emitted value sits inside that bound.
//
// RFC requirement: RFC5301-3-8 positive -- "The string is not null-terminated"
// (RFC 5301 Section 3). No NUL follows the value, and the framing that a
// NUL-terminated encoding would produce is absent from the LSP.
//
// RFC requirement: RFC5301-3-5 negative -- the length octet never counts an
// octet the value does not carry: the length+1 framing of a terminated string
// does not appear.
func TestISISHostnameTLVFraming(t *testing.T) {
	const name = "router-1.example.net"
	raw, value := emitHostnameTLV(t, name)
	if value == nil {
		t.Fatal("no TLV 137 in the originated LSP")
	}
	if packet.TLVDynamicHostname != 137 {
		t.Errorf("TLVDynamicHostname = %d, want 137", packet.TLVDynamicHostname)
	}
	if len(value) != len(name) {
		t.Errorf("TLV 137 length = %d, want %d", len(value), len(name))
	}
	if len(value) < 1 || len(value) > 255 {
		t.Errorf("TLV 137 value is %d octets, want 1..255", len(value))
	}
	if bytes.IndexByte(value, 0x00) >= 0 {
		t.Errorf("TLV 137 value %q carries a NUL octet", value)
	}
	// The framed run [137][len][value] appears in the raw LSP, which is the
	// length octet agreeing with the value it frames.
	framed := append([]byte{packet.TLVDynamicHostname, byte(len(name))}, name...)
	if !bytes.Contains(raw, framed) {
		t.Error("the raw LSP has no [137][len][value] run: the length octet disagrees with the value")
	}
	// A NUL-terminated encoding would frame one octet more.
	terminated := append([]byte{packet.TLVDynamicHostname, byte(len(name) + 1)}, append([]byte(name), 0x00)...)
	if bytes.Contains(raw, terminated) {
		t.Error("the raw LSP frames a NUL-terminated hostname")
	}
}

// TestISISHostnameEmptyOmitsTLV: an unset hostname advertises no TLV 137 at all,
// so a zero-length value is never framed.
//
// RFC requirement: RFC5301-3-6 negative -- "Value - a string of 1 to 255 bytes"
// (RFC 5301 Section 3). The lower bound is 1, so a name Ze has nothing to say
// about is omitted rather than framed as a zero-length value.
//
// RFC requirement: RFC5301-3-4 negative -- type 137 is emitted only when there
// is a hostname to advertise. The TLV is optional (RFC 5301 Section 4), so an
// unset name puts no type-137 octet on the wire.
func TestISISHostnameEmptyOmitsTLV(t *testing.T) {
	raw, value := emitHostnameTLV(t, "")
	if value != nil {
		t.Errorf("an empty hostname produced TLV 137 with value %q", value)
	}
	if bytes.Contains(raw, []byte{packet.TLVDynamicHostname, 0x00}) {
		t.Error("the raw LSP frames a zero-length TLV 137")
	}
}

// TestISISHostnameTLVTruncationUnreachable pins R-5. The 255-octet bound in
// hostnameTLV cannot be reached from config, because ISISHostnameValidator
// refuses a name longer than 255 octets before it becomes a NodeInfo. The bound
// stays as a defensive guard against a programmatic NodeInfo. This test records
// that a config-shaped value never meets it.
func TestISISHostnameTLVTruncationUnreachable(t *testing.T) {
	// The longest name a config can carry is 255 octets, and it survives whole.
	longest := strings.Repeat("c.", 127) + "d"
	if len(longest) != 255 {
		t.Fatalf("test fixture is %d octets, want 255", len(longest))
	}
	_, value := emitHostnameTLV(t, longest)
	if string(value) != longest {
		t.Errorf("a 255-octet hostname was altered on emit: got %d octets, want %d", len(value), len(longest))
	}

	// A 256-octet NodeInfo bypasses config entirely. It is an invariant
	// violation, and the guard bounds it rather than overflowing the TLV.
	_, over := emitHostnameTLV(t, strings.Repeat("d", 256))
	if len(over) != packet.MaxTLVValueLen {
		t.Errorf("a programmatic 256-octet hostname emitted %d octets, want the %d-octet bound", len(over), packet.MaxTLVValueLen)
	}
}
