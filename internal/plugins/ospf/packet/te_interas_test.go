// VALIDATES: spec-ospf-ext-2 RFC 5392 inter-AS TE sub-TLVs -- Remote AS Number (21,
// 2-byte ASN zero-extended into 4 octets), IPv4 Remote ASBR ID (22), IPv6 Remote ASBR
// ID (24, NOT 23, 16 octets); an inter-AS Link TLV omits the prohibited Link ID sub-TLV;
// boundary lengths for the fixed-width sub-TLVs.
// PREVENTS: encoding the IPv6 ASBR ID as type 23, packing a 2-byte ASN, or emitting a
// Link ID sub-TLV in an inter-AS Link TLV (RFC 5392 sec 3.2.1).
package packet

import "testing"

func TestInterAsTERemoteAsTLV(t *testing.T) {
	// A 4-byte ASN and a 2-byte ASN (zero-extended into the high 16 bits, RFC 5392 sec 3.3.1).
	for _, as := range []uint32{0, 65001, 4200000000, 0xFFFFFFFF} {
		src := TELSA{IsLink: true, Link: TELink{HasLinkType: true, LinkType: TELinkTypePointToPoint, HasRemoteAS: true, RemoteAS: as}}
		got, err := DecodeTELSA(src.Encode())
		if err != nil {
			t.Fatalf("as %d decode: %v", as, err)
		}
		if !got.Link.HasRemoteAS || got.Link.RemoteAS != as {
			t.Fatalf("remote AS = %v/%d, want %d", got.Link.HasRemoteAS, got.Link.RemoteAS, as)
		}
	}
	// Assert the 2-byte ASN 65001 occupies the low 16 bits with the high 16 zero.
	body := TELSA{IsLink: true, Link: TELink{HasRemoteAS: true, RemoteAS: 65001}}.Encode()
	// body: [Link hdr type=2 len=8][sub hdr type=21 len=4][00 00 FD E9]
	if body[4] != 0x00 || body[5] != byte(TESubRemoteAS) {
		t.Fatalf("first sub-TLV type = %d%d, want 21", body[4], body[5])
	}
	// RFC requirement: RFC5392-3.3.1-1 positive -- a 2-byte ASN is zero-extended into the 4-octet
	// Remote AS Number field: the high two octets are zero (big-endian uint32 encode) (§3.3.1).
	if body[8] != 0x00 || body[9] != 0x00 {
		t.Fatalf("2-byte ASN not zero-extended: high octets %#x %#x", body[8], body[9])
	}
}

// VALIDATES: spec-ospf-ext-2 -- TELink.IsInterAS reports true exactly when the decoded Link
// TLV carried the RFC 5392 Remote AS Number sub-TLV, and false for an ordinary intra-AS link.
// PREVENTS: an inter-AS TE link being treated as intra-AS (or vice versa) by the TE consumer.
func TestTELinkIsInterAS(t *testing.T) {
	interAS, err := DecodeTELSA(TELSA{IsLink: true, Link: TELink{
		HasLinkType: true, LinkType: TELinkTypePointToPoint, HasRemoteAS: true, RemoteAS: 65001,
	}}.Encode())
	if err != nil {
		t.Fatalf("decode inter-AS link: %v", err)
	}
	if !interAS.Link.IsInterAS() {
		t.Errorf("IsInterAS() = false for a link with the Remote AS sub-TLV, want true")
	}

	intraAS, err := DecodeTELSA(TELSA{IsLink: true, Link: TELink{
		HasLinkType: true, LinkType: TELinkTypePointToPoint, HasTEMetric: true, TEMetric: 10,
	}}.Encode())
	if err != nil {
		t.Fatalf("decode intra-AS link: %v", err)
	}
	if intraAS.Link.IsInterAS() {
		t.Errorf("IsInterAS() = true for a link without the Remote AS sub-TLV, want false")
	}
}

func TestInterAsTEIPv4AsbrId(t *testing.T) {
	src := TELSA{IsLink: true, Link: TELink{HasRemoteAS: true, RemoteAS: 65001, HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{203, 0, 113, 9}}}
	got, err := DecodeTELSA(src.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Link.HasRemoteASBRv4 || got.Link.RemoteASBRv4 != [4]byte{203, 0, 113, 9} {
		t.Fatalf("IPv4 remote ASBR ID = %v/%v", got.Link.HasRemoteASBRv4, got.Link.RemoteASBRv4)
	}
}

func TestInterAsTEIPv6AsbrIdType24(t *testing.T) {
	// RFC 5392 sec 3.3.3 / sec 6.2: the IPv6 Remote ASBR ID is sub-TLV type 24, 16 octets.
	if TESubIPv6RemoteASBRID != 24 {
		t.Fatalf("IPv6 Remote ASBR ID sub-TLV type = %d, want 24 (not 23)", TESubIPv6RemoteASBRID)
	}
	var v6 [16]byte
	for i := range v6 {
		v6[i] = byte(i + 1)
	}
	src := TELSA{IsLink: true, Link: TELink{HasRemoteAS: true, RemoteAS: 65001, HasRemoteASBRv6: true, RemoteASBRv6: v6}}
	body := src.Encode()
	got, err := DecodeTELSA(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Link.HasRemoteASBRv6 || got.Link.RemoteASBRv6 != v6 {
		t.Fatalf("IPv6 remote ASBR ID = %v/%v", got.Link.HasRemoteASBRv6, got.Link.RemoteASBRv6)
	}
	// Confirm the sub-TLV type on the wire is 24, never 23, and Length is 16.
	// RFC requirement: RFC5392-3.3.2-2 positive -- for a v4-less inter-AS link (only an IPv6
	// Remote ASBR ID present, no IPv4 one) origination emits the IPv6 Remote ASBR ID sub-TLV
	// (type 24, 16 octets), never the editorial-slip type 23 (§3.3.2, §3.3.3).
	sawV6 := false
	sawV4 := false
	it := newOpaqueTLVIterator(decodeLinkValueForTest(t, body))
	for it.Next() {
		if it.Type() == 23 {
			t.Fatalf("emitted the editorial-slip type 23 for the IPv6 Remote ASBR ID")
		}
		if it.Type() == TESubIPv4RemoteASBRID {
			sawV4 = true
		}
		if it.Type() == 24 {
			sawV6 = true
			if len(it.Value()) != 16 {
				t.Fatalf("IPv6 Remote ASBR ID length = %d, want 16", len(it.Value()))
			}
		}
	}
	if it.Err() != nil {
		t.Fatalf("sub-TLV walk: %v", it.Err())
	}
	if sawV4 {
		t.Fatalf("v4-less inter-AS link emitted an IPv4 Remote ASBR ID sub-TLV (22)")
	}
	if !sawV6 {
		t.Fatalf("IPv6 Remote ASBR ID sub-TLV (24) not present on the wire")
	}
}

func TestInterAsTEOmitsLinkID(t *testing.T) {
	// RFC 5392 sec 3.2.1: the Link ID sub-TLV MUST NOT appear in an inter-AS Link TLV.
	// An inter-AS TELink is built with HasLinkID false, so it must never encode a type-2
	// sub-TLV.
	src := TELSA{IsLink: true, Link: TELink{HasLinkType: true, LinkType: TELinkTypePointToPoint, HasRemoteAS: true, RemoteAS: 65001, HasRemoteASBRv4: true, RemoteASBRv4: [4]byte{203, 0, 113, 9}}}
	it := newOpaqueTLVIterator(decodeLinkValueForTest(t, src.Encode()))
	for it.Next() {
		if it.Type() == TESubLinkID {
			t.Fatalf("inter-AS Link TLV must not carry the Link ID sub-TLV (RFC 5392 sec 3.2.1)")
		}
	}
	if it.Err() != nil {
		t.Fatalf("sub-TLV walk: %v", it.Err())
	}
}

func TestInterAsTESubTLVLengthBoundaries(t *testing.T) {
	// Fixed-width inter-AS sub-TLVs reject a wrong length without panicking. Build a Link
	// TLV whose IPv6 Remote ASBR ID (24) carries 8 octets instead of 16.
	badV6 := linkTLVWithRawSubForTest(TESubIPv6RemoteASBRID, make([]byte, 8))
	if _, err := DecodeTELSA(badV6); err == nil {
		t.Fatalf("expected a length error for a 8-octet IPv6 Remote ASBR ID sub-TLV")
	}
	// Remote AS Number (21) with 2 octets instead of 4 is malformed.
	badAS := linkTLVWithRawSubForTest(TESubRemoteAS, make([]byte, 2))
	if _, err := DecodeTELSA(badAS); err == nil {
		t.Fatalf("expected a length error for a 2-octet Remote AS Number sub-TLV")
	}
	// A well-formed 16-octet IPv6 Remote ASBR ID decodes.
	okV6 := linkTLVWithRawSubForTest(TESubIPv6RemoteASBRID, make([]byte, 16))
	if _, err := DecodeTELSA(okV6); err != nil {
		t.Fatalf("16-octet IPv6 Remote ASBR ID rejected: %v", err)
	}
}

// decodeLinkValueForTest returns the value bytes of the top-level Link TLV in a TE body.
func decodeLinkValueForTest(t *testing.T, body []byte) []byte {
	t.Helper()
	it := newOpaqueTLVIterator(body)
	for it.Next() {
		if it.Type() == TETLVLink {
			return it.Value()
		}
	}
	t.Fatalf("no Link TLV in body")
	return nil
}

// linkTLVWithRawSubForTest builds a TE body: one Link TLV (type 2) whose value is a
// single sub-TLV with the given type and raw value bytes (padded to 4 by the builder).
func linkTLVWithRawSubForTest(subType uint16, val []byte) []byte {
	sub := []opaqueTLV{{Type: subType, Value: val}}
	inner := make([]byte, opaqueTLVsLen(sub))
	writeOpaqueTLVs(inner, sub)
	top := []opaqueTLV{{Type: TETLVLink, Value: inner}}
	out := make([]byte, opaqueTLVsLen(top))
	writeOpaqueTLVs(out, top)
	return out
}
