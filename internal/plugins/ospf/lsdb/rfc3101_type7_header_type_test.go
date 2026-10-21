// VALIDATES: RFC 3101 Section 2.3 -- a Type-7 LSA is a Type-5 LSA whose LSA header Type
// field is 7: the NSSA-LSA this router originates carries the octet 7 in the Type field of
// the header it puts on the wire, and an AS-external-LSA for the same route carries 5.
// PREVENTS: a Type-7 that is stored under the NSSA key but encoded with another header
// type, so a neighbor floods or installs it as something else.
package lsdb

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// rfc3101LSATypeOctetOffset is the offset of the LS type octet in the RFC 2328 Section A.4.1
// LSA header: LS age (2 octets), Options (1 octet), LS type (1 octet).
const rfc3101LSATypeOctetOffset = 3

// rfc3101EncodedLSA returns the wire encoding of lsa.
func rfc3101EncodedLSA(t *testing.T, lsa packet.LSA) []byte {
	t.Helper()
	buf := make([]byte, lsa.EncodedLen())
	if n := lsa.WriteTo(buf, 0); n != len(buf) {
		t.Fatalf("LSA encoded %d octets, want %d", n, len(buf))
	}
	return buf
}

// TestRFC3101Type7HeaderTypeIs7 checks the header of an originated Type-7 LSA. Goal: RFC
// 3101 Section 2.3, "The Type field in the LSA header is 7." Method: originate a Type-7 into
// NSSA 0.0.0.1, encode the stored LSA, and read the LS type octet and the decoded header.
// RFC requirement: RFC3101-2.3-4 positive -- the Type-7 LSA OriginateNSSA returns and stores
// carries LS type 7 in its header, and its wire encoding holds the octet 7 at the LS type
// offset 3, which decodes back as an NSSA-LSA.
func TestRFC3101Type7HeaderTypeIs7(t *testing.T) {
	db := newTestDB(&fakeClock{})
	router := rid("1.1.1.1")
	nssa := area("0.0.0.1")
	h, ok := db.OriginateNSSA(nssa, router, ip4("10.70.0.0"), ip4("255.255.0.0"), false, 25, ip4("10.0.0.9"), 42, false)
	if !ok {
		t.Fatalf("OriginateNSSA returned false")
	}
	if h.Type != types.LSTypeNSSA {
		t.Fatalf("originated Type-7 header carries LS type %d, want 7", h.Type)
	}
	lsa, ok := db.LookupLSA(nssa, nssaKey(ip4("10.70.0.0"), router))
	if !ok {
		t.Fatalf("Type-7 not stored in the NSSA area store")
	}
	wire := rfc3101EncodedLSA(t, lsa)
	if got := wire[rfc3101LSATypeOctetOffset]; got != 7 {
		t.Fatalf("Type-7 wire header LS type octet is %d, want 7", got)
	}
	decoded, err := packet.DecodeLSAHeader(wire)
	if err != nil {
		t.Fatalf("decode the Type-7 header: %v", err)
	}
	if decoded.Type != types.LSTypeNSSA {
		t.Fatalf("Type-7 wire header decodes as LS type %d, want 7", decoded.Type)
	}
}

// TestRFC3101Type5HeaderTypeIsNot7 checks that the header Type field is what tells a Type-7
// from the Type-5 it otherwise equals. Goal: RFC 3101 Section 2.3, "Type-7 LSAs are
// identical to Type-5 LSAs except for the following", the first being "The Type field in
// the LSA header is 7." Method: originate a Type-7 and a Type-5 for the same route and
// compare their encodings.
// RFC requirement: RFC3101-2.3-4 negative -- the AS-external-LSA OriginateExternal builds
// for the same network, metric, forwarding address and tag carries LS type 5, not 7, in
// its wire header, while its body is octet-identical to the Type-7 body.
func TestRFC3101Type5HeaderTypeIsNot7(t *testing.T) {
	db := newTestDB(&fakeClock{})
	router := rid("1.1.1.1")
	nssa := area("0.0.0.1")
	if _, ok := db.OriginateNSSA(nssa, router, ip4("10.70.0.0"), ip4("255.255.0.0"), false, 25, ip4("10.0.0.9"), 42, false); !ok {
		t.Fatalf("OriginateNSSA returned false")
	}
	if _, ok, err := db.OriginateExternal(router, ip4("10.70.0.0"), ip4("255.255.0.0"), 0, false, 25, ip4("10.0.0.9"), 42); err != nil || !ok {
		t.Fatalf("OriginateExternal: ok=%v err=%v", ok, err)
	}
	type7, ok := db.LookupLSA(nssa, nssaKey(ip4("10.70.0.0"), router))
	if !ok {
		t.Fatalf("Type-7 not stored in the NSSA area store")
	}
	type5, ok := db.LookupLSA(types.BackboneArea, types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(ip4("10.70.0.0")), AdvertisingRouter: router})
	if !ok {
		t.Fatalf("Type-5 not stored in the AS-wide store")
	}
	wire7 := rfc3101EncodedLSA(t, type7)
	wire5 := rfc3101EncodedLSA(t, type5)
	if got := wire5[rfc3101LSATypeOctetOffset]; got != 5 {
		t.Fatalf("Type-5 wire header LS type octet is %d, want 5 (never 7)", got)
	}
	if !bytes.Equal(wire5[types.LSAHeaderLen:], wire7[types.LSAHeaderLen:]) {
		t.Fatalf("Type-5 body %x differs from the Type-7 body %x for the same route", wire5[types.LSAHeaderLen:], wire7[types.LSAHeaderLen:])
	}
}
