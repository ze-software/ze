// Design: docs/architecture/wire/isis.md -- common header + PDU constant tests
package packet

import (
	"errors"
	"testing"
)

// VALIDATES: spec A-5 / Constraint -- the 9 PDU type code octets equal the
// authoritative ISO/IEC 10589 clause 9 values, NOT the research-doc sec 2
// transcription (which lists L1 LSP 0x18, L1 CSNP 0x24, L1 PSNP 0x26). A wrong
// constant causes silent interop failure with FRR/Cisco/Juniper.
// PREVENTS: a regression that swaps an L1 code and breaks adjacency/flooding.
func TestISISPDUConstants(t *testing.T) {
	want := map[PDUType]string{
		0x0f: "l1-lan-hello",
		0x10: "l2-lan-hello",
		0x11: "p2p-hello",
		0x12: "l1-lsp",
		0x14: "l2-lsp",
		0x18: "l1-csnp",
		0x19: "l2-csnp",
		0x1a: "l1-psnp",
		0x1b: "l2-psnp",
	}
	got := map[PDUType]string{
		PDUTypeL1LANHello: PDUTypeL1LANHello.String(),
		PDUTypeL2LANHello: PDUTypeL2LANHello.String(),
		PDUTypeP2PHello:   PDUTypeP2PHello.String(),
		PDUTypeL1LSP:      PDUTypeL1LSP.String(),
		PDUTypeL2LSP:      PDUTypeL2LSP.String(),
		PDUTypeL1CSNP:     PDUTypeL1CSNP.String(),
		PDUTypeL2CSNP:     PDUTypeL2CSNP.String(),
		PDUTypeL1PSNP:     PDUTypeL1PSNP.String(),
		PDUTypeL2PSNP:     PDUTypeL2PSNP.String(),
	}
	if len(got) != 9 {
		t.Fatalf("expected 9 distinct PDU type codes, got %d (duplicate value?)", len(got))
	}
	for code, name := range want {
		if got[code] != name {
			t.Errorf("PDU code %#02x = %q, want %q", code, got[code], name)
		}
	}
}

// VALIDATES: PDUType.Level maps L1/L2 PDUs to their level and reports the P2P
// Hello as level-agnostic.
func TestISISPDUTypeLevel(t *testing.T) {
	cases := []struct {
		t     PDUType
		level uint8
		ok    bool
	}{
		{PDUTypeL1LANHello, 1, true},
		{PDUTypeL2LANHello, 2, true},
		{PDUTypeL1LSP, 1, true},
		{PDUTypeL2CSNP, 2, true},
		{PDUTypeL1PSNP, 1, true},
		{PDUTypeP2PHello, 0, false}, // level-agnostic
	}
	for _, c := range cases {
		level, ok := c.t.Level()
		if level != c.level || ok != c.ok {
			t.Errorf("%v.Level() = (%d,%v), want (%d,%v)", c.t, level, ok, c.level, c.ok)
		}
	}
}

// VALIDATES: AC-1 -- the common 8-octet header encodes and decodes for all 9
// PDU types, and DecodeHeader rejects a bad discriminator, version, ID length,
// and unknown PDU type without reading past the buffer.
func TestISISHeaderRoundTrip(t *testing.T) {
	allTypes := []PDUType{
		PDUTypeL1LANHello, PDUTypeL2LANHello, PDUTypeP2PHello,
		PDUTypeL1LSP, PDUTypeL2LSP,
		PDUTypeL1CSNP, PDUTypeL2CSNP,
		PDUTypeL1PSNP, PDUTypeL2PSNP,
	}
	for _, pt := range allTypes {
		t.Run(pt.String(), func(t *testing.T) {
			buf := make([]byte, CommonHeaderLen)
			n := writeCommonHeader(buf, 0, pt, CommonHeaderLen, 0)
			if n != CommonHeaderLen {
				t.Fatalf("writeCommonHeader returned %d, want %d", n, CommonHeaderLen)
			}
			h, bodyOff, err := DecodeHeader(buf)
			if err != nil {
				t.Fatalf("DecodeHeader: %v", err)
			}
			if h.PDUType != pt {
				t.Errorf("PDUType = %v, want %v", h.PDUType, pt)
			}
			if bodyOff != CommonHeaderLen {
				t.Errorf("bodyOff = %d, want %d", bodyOff, CommonHeaderLen)
			}
			if h.IDLength != IDLength {
				t.Errorf("IDLength = %d, want %d", h.IDLength, IDLength)
			}
		})
	}
}

// VALIDATES: AC-1, AC-11 -- DecodeHeader rejects malformed headers with typed
// errors and never panics.
func TestISISHeaderRejects(t *testing.T) {
	good := make([]byte, CommonHeaderLen)
	writeCommonHeader(good, 0, PDUTypeL1LSP, CommonHeaderLen, 0)

	cases := []struct {
		name   string
		mutate func([]byte)
		want   error
	}{
		{"short", func(_ []byte) {}, ErrShortBuffer}, // handled via slice below
		{"bad-discriminator", func(b []byte) { b[offDiscriminator] = 0x82 }, ErrBadDiscriminator},
		{"bad-version-proto-ext", func(b []byte) { b[offVersionProtoExt] = 0x02 }, ErrBadVersion},
		{"bad-version", func(b []byte) { b[offVersion] = 0x02 }, ErrBadVersion},
		{"bad-id-length", func(b []byte) { b[offIDLength] = 7 }, ErrBadIDLength},
		{"unknown-pdu-type", func(b []byte) { b[offPDUType] = 0x1f }, ErrUnknownPDUType},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "short" {
				if _, _, err := DecodeHeader(good[:CommonHeaderLen-1]); !errors.Is(err, c.want) {
					t.Errorf("short header: err = %v, want %v", err, c.want)
				}
				return
			}
			b := make([]byte, CommonHeaderLen)
			copy(b, good)
			c.mutate(b)
			if _, _, err := DecodeHeader(b); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// VALIDATES: ID length 0 (the on-wire shorthand for 6) is accepted.
func TestISISHeaderIDLengthZero(t *testing.T) {
	b := make([]byte, CommonHeaderLen)
	writeCommonHeader(b, 0, PDUTypeL1LSP, CommonHeaderLen, 0)
	b[offIDLength] = 0 // shorthand for 6
	h, _, err := DecodeHeader(b)
	if err != nil {
		t.Fatalf("ID length 0 should be accepted: %v", err)
	}
	if h.IDLength != 0 {
		t.Errorf("IDLength = %d, want 0 (as received)", h.IDLength)
	}
}
