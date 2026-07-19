// VALIDATES: spec-ospf-ext-4 -- the RFC 7684 Extended Link Opaque LSA body codec: the
// Extended Link TLV Link Type / Reserved / Link ID / Link Data + sub-TLV region round-trip
// (AC-3/AC-12), and decode uses the FIRST Extended Link TLV while counting extras (AC-8, §3.1
// SHALL: only one per LSA).
// PREVENTS: a second Extended Link TLV silently replacing the first, or a lost sub-TLV region.
package packet

import (
	"bytes"
	"testing"
)

func TestExtLinkTLVRoundTrip(t *testing.T) {
	in := ExtLinkTLV{
		LinkType: 1, // RFC 2328 point-to-point
		LinkID:   [4]byte{2, 2, 2, 2},
		LinkData: [4]byte{10, 0, 0, 1},
		SubTLVs:  []ExtSubTLV{{Type: 2, Value: []byte{1, 2, 3, 4, 5}}},
	}
	body := EncodeExtLinkLSA(in)
	if len(body)%4 != 0 {
		t.Fatalf("body length %d not 4-octet aligned", len(body))
	}
	out, err := DecodeExtLinkLSA(body)
	// RFC requirement: RFC7684-5-1 positive -- a well-formed Extended Link body decodes without
	// error through the bound-checked decoder (DecodeExtLinkLSA, packet/ext_link.go:78), proving
	// the malformed-detection bounds do not reject valid input.
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.HasLink {
		t.Fatalf("no Extended Link TLV decoded")
	}
	if out.Link.LinkType != 1 || out.Link.LinkID != [4]byte{2, 2, 2, 2} || out.Link.LinkData != [4]byte{10, 0, 0, 1} {
		t.Fatalf("link fields = %+v", out.Link)
	}
	if len(out.Link.SubTLVs) != 1 || out.Link.SubTLVs[0].Type != 2 || !bytes.Equal(out.Link.SubTLVs[0].Value, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("sub-tlvs = %+v", out.Link.SubTLVs)
	}
	if out.ExtraLinkTLVs != 0 {
		t.Fatalf("extra link TLVs = %d, want 0", out.ExtraLinkTLVs)
	}
	if again := EncodeExtLinkLSA(out.Link); !bytes.Equal(again, body) {
		t.Fatalf("re-encode mismatch")
	}
}

func TestExtLinkEmptyContainerRoundTrip(t *testing.T) {
	in := ExtLinkTLV{LinkType: 2, LinkID: [4]byte{10, 0, 0, 1}, LinkData: [4]byte{10, 0, 0, 2}}
	body := EncodeExtLinkLSA(in)
	out, err := DecodeExtLinkLSA(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.HasLink || len(out.Link.SubTLVs) != 0 {
		t.Fatalf("empty container decode = %+v", out)
	}
	// 4-byte TLV header + 12-byte fixed value = 16 octets.
	if len(body) != 16 {
		t.Fatalf("empty container body = %d octets, want 16", len(body))
	}
}

func TestExtLinkSingleTLVEnforced(t *testing.T) {
	// §3.1 SHALL: only one Extended Link TLV per LSA. Two type-1 TLVs -> decode uses the
	// first and counts the extra so the consumer can log it (AC-8).
	first := ExtLinkTLV{LinkType: 1, LinkID: [4]byte{2, 2, 2, 2}, LinkData: [4]byte{10, 0, 0, 1}}
	second := ExtLinkTLV{LinkType: 2, LinkID: [4]byte{3, 3, 3, 3}, LinkData: [4]byte{10, 0, 0, 5}}
	body := append(EncodeExtLinkLSA(first), EncodeExtLinkLSA(second)...)
	out, err := DecodeExtLinkLSA(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// RFC requirement: RFC7684-3.1-1 negative -- an LSA carrying two Extended Link TLVs decodes
	// to the FIRST TLV only and counts the extra (ExtraLinkTLVs=1) so the consumer can log it;
	// a second TLV never silently replaces the first (§3.1 SHALL: only one per LSA).
	if out.Link.LinkID != [4]byte{2, 2, 2, 2} {
		t.Fatalf("must use FIRST Extended Link TLV, got %+v", out.Link)
	}
	if out.ExtraLinkTLVs != 1 {
		t.Fatalf("extra link TLVs = %d, want 1", out.ExtraLinkTLVs)
	}
}
